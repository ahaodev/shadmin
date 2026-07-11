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
	"time"
)

// SyncService casbin同步服务
type SyncService struct {
	entClient *ent.Client
	manager   Manager
}

// NewSyncService 创建新的同步服务实例
func NewSyncService(entClient *ent.Client, manager Manager) *SyncService {
	return &SyncService{
		entClient: entClient,
		manager:   manager,
	}
}

// SyncFromDatabase 从数据库同步所有casbin数据
// 这是主要的同步方法，会清空现有的casbin数据并重新从数据库加载
func (s *SyncService) SyncFromDatabase(ctx context.Context) error {
	logger.Info("开始从数据库同步casbin数据")

	startTime := time.Now()

	// 1. 清空现有的casbin策略
	if err := s.clearCasbinPolicies(ctx); err != nil {
		return fmt.Errorf("清空casbin策略失败: %w", err)
	}

	// 2. 同步用户角色关系
	if err := s.syncUserRoles(ctx); err != nil {
		return fmt.Errorf("同步用户角色关系失败: %w", err)
	}

	// 3. 同步角色权限策略
	if err := s.syncRolePermissions(ctx); err != nil {
		return fmt.Errorf("同步角色权限策略失败: %w", err)
	}

	// 4. 保存策略到数据库
	if err := s.manager.SavePolicy(); err != nil {
		return fmt.Errorf("保存casbin策略失败: %w", err)
	}

	duration := time.Since(startTime)
	logger.Infof("同步完成，耗时: %v", duration)

	return nil
}

// SyncIncremental 基于 updated_at 差异同步最近变化的用户、角色、菜单与 API 资源。
// 删除类变更依赖 Ent Hook 定向同步兜底；没有变更日志表时，定时任务只处理可由时间戳识别的增量。
func (s *SyncService) SyncIncremental(ctx context.Context, since time.Time) error {
	logger.Infof("开始增量同步casbin数据，since=%s", since.Format(time.RFC3339Nano))

	startTime := time.Now()
	var errs []error

	userIDs, err := s.changedUserIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("查询变更用户失败: %w", err)
	}

	roleIDs, err := s.changedRoleIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("查询变更角色失败: %w", err)
	}

	menuIDs, err := s.changedMenuIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("查询变更菜单失败: %w", err)
	}
	apiResourceIDs, err := s.changedAPIResourceIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("查询变更API资源失败: %w", err)
	}

	menuRoleIDs, err := s.roleIDsForMenus(ctx, menuIDs)
	if err != nil {
		return fmt.Errorf("查询菜单关联角色失败: %w", err)
	}
	apiResourceRoleIDs, err := s.roleIDsForAPIResources(ctx, apiResourceIDs)
	if err != nil {
		return fmt.Errorf("查询API资源关联角色失败: %w", err)
	}
	roleIDs = uniqueStrings(append(roleIDs, append(menuRoleIDs, apiResourceRoleIDs...)...))
	userIDs = uniqueStrings(userIDs)

	for _, userID := range userIDs {
		if err := s.SyncUserRole(ctx, userID); err != nil {
			logger.WithError(err).Warnf("增量同步用户角色失败: user=%s", userID)
			errs = append(errs, fmt.Errorf("同步用户 %s 失败: %w", userID, err))
		}
	}

	for _, roleID := range roleIDs {
		if err := s.SyncRolePermissions(ctx, roleID); err != nil {
			logger.WithError(err).Warnf("增量同步角色权限失败: role=%s", roleID)
			errs = append(errs, fmt.Errorf("同步角色 %s 失败: %w", roleID, err))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	duration := time.Since(startTime)
	logger.Infof("增量同步完成，用户: %d, 角色: %d, 菜单: %d, API资源: %d, 耗时: %v",
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

// clearCasbinPolicies 清空所有casbin策略
func (s *SyncService) clearCasbinPolicies(_ context.Context) error {
	logger.Info("清空现有casbin策略")

	// 清空所有权限策略 (p规则)：收集唯一 sub，每个 sub 一次批量删除
	subs := make(map[string]struct{})
	for _, policy := range s.manager.GetAllPolicies() {
		if len(policy) >= 1 {
			subs[policy[0]] = struct{}{}
		}
	}
	for sub := range subs {
		if _, err := s.manager.RemoveFilteredPolicy(0, sub); err != nil {
			logger.WithField("sub", sub).Warnf("批量移除策略失败: %v", err)
		}
	}

	// 清空所有角色映射 (g规则)：收集唯一 user，每人一次批量删除
	users := make(map[string]struct{})
	for _, roleMapping := range s.manager.GetAllRoles() {
		if len(roleMapping) >= 1 {
			users[roleMapping[0]] = struct{}{}
		}
	}
	for uid := range users {
		if _, err := s.manager.DeleteRolesForUser(uid); err != nil {
			logger.WithField("user", uid).Warnf("批量移除角色映射失败: %v", err)
		}
	}

	logger.Info("清空策略完成")
	return nil
}

// syncUserRoles 同步用户角色关系
func (s *SyncService) syncUserRoles(ctx context.Context) error {
	logger.Info("开始同步用户角色关系")

	// 查询所有活跃用户及其角色
	users, err := s.entClient.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)

	if err != nil {
		return fmt.Errorf("查询用户角色关系失败: %w", err)
	}

	userRoleCount := 0

	for _, u := range users {
		for _, r := range u.Edges.Roles {
			if _, err := s.manager.AddRoleForUser(u.ID, r.ID); err != nil {
				logger.WithError(err).Warnf("添加用户角色失败: user=%s, role=%s", u.ID, r.ID)
				continue
			}
			userRoleCount++
		}
	}

	logger.Infof("同步用户角色关系完成，共处理 %d 条关系", userRoleCount)
	return nil
}

// syncRolePermissions 同步角色权限策略
func (s *SyncService) syncRolePermissions(ctx context.Context) error {
	logger.Info("开始同步角色权限策略")

	// 查询所有活跃角色及其菜单和API资源
	roles, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.Where().
				WithAPIResources(func(ar *ent.ApiResourceQuery) {
					ar.Where(apiresource.IsPublicEQ(false)) // 只同步需要权限验证的API
				})
		}).
		All(ctx)

	if err != nil {
		return fmt.Errorf("查询角色权限关系失败: %w", err)
	}

	policyCount := 0
	for _, r := range roles {
		n, err := s.applyRolePolicies(r.ID, r.Name, r.Edges.Menus)
		if err != nil {
			return err
		}
		policyCount += n
	}

	logger.Infof("同步角色权限策略完成，共处理 %d 条策略", policyCount)
	return nil
}

// applyRolePolicies 将角色的权限写入 Casbin。
// admin 角色写入通配符策略；其余角色按关联菜单的 API 资源写入。
// 返回实际写入的策略条数。
func (s *SyncService) applyRolePolicies(roleID, roleName string, menus []*ent.Menu) (int, error) {
	log := logger.WithField("role", roleID)

	if roleName == "admin" {
		if _, err := s.manager.AddPolicy(roleID, "*", "*"); err != nil {
			log.WithError(err).Warn("添加admin通配符权限失败")
			return 0, nil
		}
		log.Info("已为admin角色添加通配符权限")
		return 1, nil
	}

	count := 0
	for _, menu := range menus {
		for _, apiRes := range menu.Edges.APIResources {
			if _, err := s.manager.AddPolicy(roleID, apiRes.Path, apiRes.Method); err != nil {
				log.WithError(err).Warnf("添加API权限失败: %s %s", apiRes.Method, apiRes.Path)
				continue
			}
			count++
		}
	}
	return count, nil
}

// SyncUserRole 同步单个用户的角色关系
func (s *SyncService) SyncUserRole(ctx context.Context, userID string) error {
	logger.Infof("同步用户角色: %s", userID)

	// 清除用户现有的所有角色
	if _, err := s.manager.DeleteRolesForUser(userID); err != nil {
		logger.WithError(err).Warnf("清除用户角色失败: user=%s", userID)
	}

	// 查询用户的当前角色（仅活跃用户有效）
	u, err := s.entClient.User.Query().
		Where(user.IDEQ(userID), user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return s.manager.SavePolicy()
		}
		return fmt.Errorf("查询用户角色失败: %w", err)
	}

	// 重新添加用户角色
	for _, r := range u.Edges.Roles {
		if _, err := s.manager.AddRoleForUser(userID, r.ID); err != nil {
			logger.WithError(err).Warnf("添加用户角色失败: user=%s, role=%s", userID, r.ID)
		}
	}

	return s.manager.SavePolicy()
}

// SyncRolePermissions 同步单个角色的权限策略
func (s *SyncService) SyncRolePermissions(ctx context.Context, roleID string) error {
	logger.Infof("同步角色权限: %s", roleID)

	// 清除角色现有权限策略
	if err := s.clearRolePolicies(ctx, roleID); err != nil {
		return fmt.Errorf("清除角色现有权限失败: %w", err)
	}

	// 查询角色的当前菜单和API资源
	r, err := s.entClient.Role.Query().
		Where(role.IDEQ(roleID), role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.Where().
				WithAPIResources(func(ar *ent.ApiResourceQuery) {
					ar.Where(apiresource.IsPublicEQ(false))
				})
		}).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return s.manager.SavePolicy()
		}
		return fmt.Errorf("查询角色权限失败: %w", err)
	}

	if _, err := s.applyRolePolicies(roleID, r.Name, r.Edges.Menus); err != nil {
		return err
	}

	return s.manager.SavePolicy()
}

// clearRolePolicies 清除指定角色的所有权限策略
func (s *SyncService) clearRolePolicies(_ context.Context, roleID string) error {
	if _, err := s.manager.RemoveFilteredPolicy(0, roleID); err != nil {
		logger.WithField("role", roleID).Warnf("移除角色策略失败: %v", err)
	}
	return nil
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
		return nil, err
	}

	roleIDs := make([]string, 0)
	for _, m := range menus {
		for _, r := range m.Edges.Roles {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	return uniqueStrings(roleIDs), nil
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
		return nil, err
	}

	roleIDs := make([]string, 0)
	for _, m := range menus {
		for _, r := range m.Edges.Roles {
			roleIDs = append(roleIDs, r.ID)
		}
	}
	return uniqueStrings(roleIDs), nil
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

// GetSyncStats 获取同步统计信息
func (s *SyncService) GetSyncStats(ctx context.Context) (*SyncStats, error) {
	stats := &SyncStats{}

	// 统计数据库中活跃的 user-role 对数（不是用户数）
	users, err := s.entClient.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计用户角色数量失败: %w", err)
	}
	userRoleCount := 0
	for _, u := range users {
		userRoleCount += len(u.Edges.Roles)
	}

	rolePermCount, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus().
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计角色权限数量失败: %w", err)
	}

	// 统计casbin中的数据
	stats.DatabaseUserRoles = userRoleCount
	stats.DatabaseRolePermissions = rolePermCount
	stats.CasbinRoles = len(s.manager.GetAllRoles())
	stats.CasbinPolicies = len(s.manager.GetAllPolicies())

	return stats, nil
}

// SyncStats 同步统计信息
type SyncStats struct {
	DatabaseUserRoles       int `json:"database_user_roles"`
	DatabaseRolePermissions int `json:"database_role_permissions"`
	CasbinRoles             int `json:"casbin_roles"`
	CasbinPolicies          int `json:"casbin_policies"`
}

// IsHealthy 检查同步状态是否健康
func (stats *SyncStats) IsHealthy() bool {
	// 简单的健康检查：casbin中的数据不应该为空
	return stats.CasbinRoles > 0 && stats.CasbinPolicies > 0
}
