package casbin

import (
	"context"
	"errors"
	"fmt"
	"shadmin/ent"
	"shadmin/ent/apiresource"
	"shadmin/ent/menu"
	"shadmin/ent/role"
	"shadmin/ent/user"
	"sync"
	"time"
)

// SyncService casbin sync service
type SyncService struct {
	entClient *ent.Client
	manager   Manager
	mu        sync.Mutex
}

// NewSyncService creates a new sync service instance
func NewSyncService(entClient *ent.Client, manager Manager) *SyncService {
	return &SyncService{
		entClient: entClient,
		manager:   manager,
	}
}

// SyncFromDatabase syncs all Casbin data from the database.
func (s *SyncService) SyncFromDatabase(ctx context.Context) error {
	// 全量重建（清空 + 重写 + SavePolicy）必须整体互斥，否则可能被 hook 触发的定向同步穿插。
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Info("Starting Casbin data sync from database")

	startTime := time.Now()

	// 1. Clear existing Casbin policies.
	if err := s.clearCasbinPolicies(); err != nil {
		return fmt.Errorf("failed to clear existing casbin policies: %w", err)
	}

	// 2. Sync user-role relationships
	if err := s.syncUserRoles(ctx); err != nil {
		return fmt.Errorf("failed to sync user-role relationships: %w", err)
	}

	// 3. Sync role permission policies
	if err := s.syncRolePermissions(ctx); err != nil {
		return fmt.Errorf("failed to sync role permission policies: %w", err)
	}

	// 4. Save policies to the database
	if err := s.manager.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save Casbin policies: %w", err)
	}

	logger.Infof("Sync completed, elapsed: %v", time.Since(startTime))
	return nil
}

// SyncIncremental syncs recently changed users, roles, menus, and API resources based on updated_at.
// Delete changes rely on Ent hook fallback; without a change log table, the scheduler only processes changes identifiable by timestamps.
func (s *SyncService) SyncIncremental(ctx context.Context, since time.Time) error {
	logger.Infof("Starting incremental Casbin data sync, since=%s", since.Format(time.RFC3339Nano))

	startTime := time.Now()
	var errs []error

	userIDs, err := s.changedUserIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to query changed users: %w", err)
	}

	roleIDs, err := s.changedRoleIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to query changed roles: %w", err)
	}

	menuIDs, err := s.changedMenuIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to query changed menus: %w", err)
	}
	apiResourceIDs, err := s.changedAPIResourceIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to query changed API resources: %w", err)
	}

	menuRoleIDs, err := s.roleIDsForMenus(ctx, menuIDs)
	if err != nil {
		return fmt.Errorf("failed to query roles associated with menus: %w", err)
	}
	apiResourceRoleIDs, err := s.roleIDsForAPIResources(ctx, apiResourceIDs)
	if err != nil {
		return fmt.Errorf("failed to query roles associated with API resources: %w", err)
	}
	roleIDs = uniqueStrings(append(roleIDs, append(menuRoleIDs, apiResourceRoleIDs...)...))
	userIDs = uniqueStrings(userIDs)

	for _, userID := range userIDs {
		if err := s.SyncUserRole(ctx, userID); err != nil {
			logger.WithError(err).Warnf("Incremental sync user role failed: user=%s", userID)
			errs = append(errs, fmt.Errorf("failed to sync user %s: %w", userID, err))
		}
	}

	for _, roleID := range roleIDs {
		if err := s.SyncRolePermissions(ctx, roleID); err != nil {
			logger.WithError(err).Warnf("Incremental sync role permissions failed: role=%s", roleID)
			errs = append(errs, fmt.Errorf("failed to sync role %s: %w", roleID, err))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	duration := time.Since(startTime)
	logger.Infof("Incremental sync completed, users: %d, roles: %d, menus: %d, API resources: %d, elapsed: %v",
		len(userIDs), len(roleIDs), len(menuIDs), len(apiResourceIDs), duration)
	return nil
}

func (s *SyncService) changedUserIDs(ctx context.Context, since time.Time) ([]string, error) {
	return s.entClient.User.Query().
		Where(user.UpdatedAtGTE(since)).
		Select(user.FieldID).
		Strings(ctx)
}

func (s *SyncService) changedRoleIDs(ctx context.Context, since time.Time) ([]string, error) {
	return s.entClient.Role.Query().
		Where(role.UpdatedAtGTE(since)).
		Select(role.FieldID).
		Strings(ctx)
}

func (s *SyncService) changedMenuIDs(ctx context.Context, since time.Time) ([]string, error) {
	return s.entClient.Menu.Query().
		Where(menu.UpdatedAtGTE(since)).
		Select(menu.FieldID).
		Strings(ctx)
}

func (s *SyncService) changedAPIResourceIDs(ctx context.Context, since time.Time) ([]string, error) {
	return s.entClient.ApiResource.Query().
		Where(apiresource.UpdatedAtGTE(since)).
		Select(apiresource.FieldID).
		Strings(ctx)
}

// clearCasbinPolicies clears all Casbin policies.
func (s *SyncService) clearCasbinPolicies() error {
	logger.Info("Clearing existing Casbin policies")

	var errs []error

	// Clear all permission policies (p rules): collect unique subs and delete them in batches
	subs := make(map[string]struct{})
	for _, policy := range s.manager.GetAllPolicies() {
		if len(policy) >= 1 {
			subs[policy[0]] = struct{}{}
		}
	}
	for sub := range subs {
		if _, err := s.manager.RemoveFilteredPolicy(0, sub); err != nil {
			logger.WithField("sub", sub).Warnf("Failed to remove policies in batch: %v", err)
			errs = append(errs, fmt.Errorf("failed to remove policies for subject %s: %w", sub, err))
		}
	}

	// Clear all role mappings (g rules): collect unique users and delete them in batches
	users := make(map[string]struct{})
	for _, roleMapping := range s.manager.GetAllRoles() {
		if len(roleMapping) >= 1 {
			users[roleMapping[0]] = struct{}{}
		}
	}
	for uid := range users {
		if _, err := s.manager.DeleteRolesForUser(uid); err != nil {
			logger.WithField("user", uid).Warnf("Failed to remove role mappings in batch: %v", err)
			errs = append(errs, fmt.Errorf("failed to remove role mappings for user %s: %w", uid, err))
		}
	}

	logger.Info("Cleared policies")
	return errors.Join(errs...)
}

// syncUserRoles syncs user-role relationships
func (s *SyncService) syncUserRoles(ctx context.Context) error {
	logger.Info("Starting user-role relationship sync")

	// Query all active users and their roles
	users, err := s.entClient.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to query user-role relationships: %w", err)
	}

	userRoleCount := 0
	var errs []error

	for _, u := range users {
		for _, r := range u.Edges.Roles {
			if _, err := s.manager.AddRoleForUser(u.ID, r.ID); err != nil {
				logger.WithError(err).Warnf("Failed to add user role: user=%s, role=%s", u.ID, r.ID)
				errs = append(errs, fmt.Errorf("failed to add role %s for user %s: %w", r.ID, u.ID, err))
				continue
			}
			userRoleCount++
		}
	}

	logger.Infof("Completed user-role sync, processed %d relationships", userRoleCount)
	return errors.Join(errs...)
}

// syncRolePermissions syncs role permission policies
func (s *SyncService) syncRolePermissions(ctx context.Context) error {
	logger.Info("Starting role permission policy sync")

	// Query all active roles and their menus and API resources
	roles, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.WithAPIResources()
		}).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to query role permission relationships: %w", err)
	}

	policyCount := 0
	var errs []error
	for _, r := range roles {
		n, err := s.applyRolePolicies(r.ID, r.Name, r.Edges.Menus)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to apply policies for role %s: %w", r.ID, err))
		}
		policyCount += n
	}

	logger.Infof("Completed role permission policy sync, processed %d policies", policyCount)
	return errors.Join(errs...)
}

// applyRolePolicies 把一个角色的权限写进 Casbin，返回实际写入条数与聚合的写失败。
func (s *SyncService) applyRolePolicies(roleID, roleName string, menus []*ent.Menu) (int, error) {
	log := logger.WithField("role", roleID)

	count := 0
	var errs []error
	for _, rule := range desiredRolePolicies(roleName, menus) {
		if _, err := s.manager.AddPolicy(roleID, rule.obj, rule.act); err != nil {
			log.WithError(err).Warnf("Failed to add API permission: %s %s", rule.act, rule.obj)
			errs = append(errs, fmt.Errorf("failed to add policy %s %s for role %s: %w", rule.act, rule.obj, roleID, err))
			continue
		}
		count++
	}
	return count, errors.Join(errs...)
}

// policyRule 是 p 规则里除 subject 之外的部分。
type policyRule struct {
	obj string
	act string
}

// desiredRolePolicies 计算角色应有的 p 规则集合（不含 subject）：
func desiredRolePolicies(roleName string, menus []*ent.Menu) []policyRule {
	if roleName == "admin" {
		return []policyRule{{obj: "*", act: "*"}}
	}

	rules := make([]policyRule, 0)
	seen := make(map[policyRule]struct{})
	for _, menu := range menus {
		for _, apiRes := range menu.Edges.APIResources {
			if apiRes.IsPublic { // 公开资源不经鉴权，不进策略
				continue
			}
			rule := policyRule{obj: apiRes.Path, act: apiRes.Method}
			if _, ok := seen[rule]; ok {
				continue
			}
			seen[rule] = struct{}{}
			rules = append(rules, rule)
		}
	}
	return rules
}

// diffKeys 返回 toAdd = desired \ current、toRemove = current \ desired；均去重并保持入参顺序。
func diffKeys[K comparable](current, desired []K) (toAdd, toRemove []K) {
	inCurrent := make(map[K]struct{}, len(current))
	for _, k := range current {
		inCurrent[k] = struct{}{}
	}
	inDesired := make(map[K]struct{}, len(desired))
	for _, k := range desired {
		inDesired[k] = struct{}{}
	}

	seenAdd := make(map[K]struct{}, len(desired))
	for _, k := range desired {
		if _, ok := inCurrent[k]; ok {
			continue
		}
		if _, ok := seenAdd[k]; ok {
			continue
		}
		seenAdd[k] = struct{}{}
		toAdd = append(toAdd, k)
	}

	seenRemove := make(map[K]struct{}, len(current))
	for _, k := range current {
		if _, ok := inDesired[k]; ok {
			continue
		}
		if _, ok := seenRemove[k]; ok {
			continue
		}
		seenRemove[k] = struct{}{}
		toRemove = append(toRemove, k)
	}

	return toAdd, toRemove
}

// currentRolePolicies 提取某角色当前的 (obj, act) 策略集合。
func currentRolePolicies(m Manager, roleID string) []policyRule {
	rules := make([]policyRule, 0)
	for _, row := range m.GetFilteredPolicies(0, roleID) {
		if len(row) < 3 { // p 规则形如 [sub, obj, act]
			continue
		}
		rules = append(rules, policyRule{obj: row[1], act: row[2]})
	}
	return rules
}

// SyncUserRole syncs a single user's role relationships, applying only the diff between
func (s *SyncService) SyncUserRole(ctx context.Context, userID string) error {
	logger.Infof("Syncing user roles: %s", userID)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 只认活跃用户：缺失或非活跃 → desired 为空，即撤销其全部角色。
	desired := make([]string, 0)
	u, err := s.entClient.User.Query().
		Where(user.IDEQ(userID), user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		Only(ctx)
	switch {
	case err == nil:
		for _, r := range u.Edges.Roles {
			desired = append(desired, r.ID)
		}
	case !ent.IsNotFound(err):
		return fmt.Errorf("failed to query user roles: %w", err)
	}

	return s.applyUserRoleDiff(userID, desired)
}

// applyUserRoleDiff 把用户的 g 规则调整为 desired，只增删差集。不触碰 DB，便于单测。
func (s *SyncService) applyUserRoleDiff(userID string, desired []string) error {
	current := s.manager.GetRolesForUser(userID)
	toAdd, toRemove := diffKeys(current, desired)

	var errs []error
	for _, roleID := range toAdd {
		if _, err := s.manager.AddRoleForUser(userID, roleID); err != nil {
			logger.WithError(err).Warnf("Failed to add user role: user=%s, role=%s", userID, roleID)
			errs = append(errs, fmt.Errorf("failed to add role %s for user %s: %w", roleID, userID, err))
		}
	}
	for _, roleID := range toRemove {
		if _, err := s.manager.DeleteRoleForUser(userID, roleID); err != nil {
			logger.WithError(err).Warnf("Failed to delete user role: user=%s, role=%s", userID, roleID)
			errs = append(errs, fmt.Errorf("failed to delete role %s for user %s: %w", roleID, userID, err))
		}
	}

	return errors.Join(errs...)
}

// SyncRolePermissions syncs a single role's permission policies, applying only the diff
func (s *SyncService) SyncRolePermissions(ctx context.Context, roleID string) error {
	logger.Infof("Syncing role permissions: %s", roleID)

	s.mu.Lock()
	defer s.mu.Unlock()

	desired := make([]policyRule, 0)
	r, err := s.entClient.Role.Query().
		Where(role.IDEQ(roleID), role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.WithAPIResources()
		}).
		Only(ctx)
	switch {
	case err == nil:
		desired = desiredRolePolicies(r.Name, r.Edges.Menus)
	case !ent.IsNotFound(err):
		return fmt.Errorf("failed to query role permissions: %w", err)
	}

	return s.applyRolePolicyDiff(roleID, desired)
}

// applyRolePolicyDiff 把角色的 p 策略调整为 desired，只增删差集。不触碰 DB，便于单测。
func (s *SyncService) applyRolePolicyDiff(roleID string, desired []policyRule) error {
	current := currentRolePolicies(s.manager, roleID)
	toAdd, toRemove := diffKeys(current, desired)

	var errs []error
	for _, rule := range toAdd {
		if _, err := s.manager.AddPolicy(roleID, rule.obj, rule.act); err != nil {
			logger.WithError(err).Warnf("Failed to add API permission: %s %s", rule.act, rule.obj)
			errs = append(errs, fmt.Errorf("failed to add policy %s %s for role %s: %w", rule.act, rule.obj, roleID, err))
		}
	}
	for _, rule := range toRemove {
		if _, err := s.manager.RemovePolicy(roleID, rule.obj, rule.act); err != nil {
			logger.WithError(err).Warnf("Failed to remove API permission: %s %s", rule.act, rule.obj)
			errs = append(errs, fmt.Errorf("failed to remove policy %s %s for role %s: %w", rule.act, rule.obj, roleID, err))
		}
	}

	return errors.Join(errs...)
}

func (s *SyncService) RoleIDsForMenus(ctx context.Context, menuIDs []string) ([]string, error) {
	return s.roleIDsForMenus(ctx, menuIDs)
}

func (s *SyncService) RoleIDsForAPIResources(ctx context.Context, apiResourceIDs []string) ([]string, error) {
	return s.roleIDsForAPIResources(ctx, apiResourceIDs)
}

func (s *SyncService) roleIDsForMenus(ctx context.Context, menuIDs []string) ([]string, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}

	menus, err := s.entClient.Menu.Query().
		Where(menu.IDIn(menuIDs...)).
		WithRoles().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query menus by IDs: %w", err)
	}
	return uniqueRoleIDs(menus), nil
}

func (s *SyncService) roleIDsForAPIResources(ctx context.Context, apiResourceIDs []string) ([]string, error) {
	if len(apiResourceIDs) == 0 {
		return nil, nil
	}

	menus, err := s.entClient.Menu.Query().
		Where(menu.HasAPIResourcesWith(apiresource.IDIn(apiResourceIDs...))).
		WithRoles().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query menus by API resources: %w", err)
	}
	return uniqueRoleIDs(menus), nil
}

// uniqueRoleIDs 汇总 menus 关联的角色 ID（去重、去空）。
func uniqueRoleIDs(menus []*ent.Menu) []string {
	roleIDs := make([]string, 0)
	for _, m := range menus {
		for _, r := range m.Edges.Roles {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	return uniqueStrings(roleIDs)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

// MergeUniqueIDs 将 src 去重后合并进 dst，丢弃空串；src 为空时原样返回 dst。
func MergeUniqueIDs(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	return uniqueStrings(append(dst, src...))
}

// GetSyncStats gets sync statistics
func (s *SyncService) GetSyncStats(ctx context.Context) (*SyncStats, error) {
	stats := &SyncStats{}

	userRoleCount, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		QueryUsers().
		Where(user.StatusEQ("active")).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count user-role relationships: %w", err)
	}

	roleCount, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active roles: %w", err)
	}

	// Count the data in Casbin
	stats.DatabaseUserRoles = userRoleCount
	stats.DatabaseRoles = roleCount
	stats.CasbinRoles = len(s.manager.GetAllRoles())
	stats.CasbinPolicies = len(s.manager.GetAllPolicies())

	return stats, nil
}

// SyncStats sync statistics
type SyncStats struct {
	DatabaseUserRoles int `json:"database_user_roles"`
	DatabaseRoles     int `json:"database_roles"`
	CasbinRoles       int `json:"casbin_roles"`
	CasbinPolicies    int `json:"casbin_policies"`
}

// IsHealthy 只报它真正能判断的一件事：DB 侧为空时 Casbin 不该有残留（有残留说明清理不完整）。
func (stats *SyncStats) IsHealthy() bool {
	if stats.DatabaseUserRoles != 0 || stats.DatabaseRoles != 0 {
		return true
	}
	return stats.CasbinRoles == 0 && stats.CasbinPolicies == 0
}
