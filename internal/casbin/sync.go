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

// SyncService casbin sync service
type SyncService struct {
	entClient *ent.Client
	manager   Manager
}

// NewSyncService creates a new sync service instance
func NewSyncService(entClient *ent.Client, manager Manager) *SyncService {
	return &SyncService{
		entClient: entClient,
		manager:   manager,
	}
}

// SyncFromDatabase syncs all Casbin data from the database.
// This is the primary sync method; it clears existing Casbin data and reloads it from the database.
func (s *SyncService) SyncFromDatabase(ctx context.Context) error {
	logger.Info("Starting Casbin data sync from database")

	startTime := time.Now()

	// 1. Clear existing Casbin policies
	if err := s.clearCasbinPolicies(ctx); err != nil {
		return fmt.Errorf("failed to clear Casbin policies: %w", err)
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

	duration := time.Since(startTime)
	logger.Infof("Sync completed, elapsed: %v", duration)

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

// clearCasbinPolicies clears all Casbin policies
func (s *SyncService) clearCasbinPolicies(_ context.Context) error {
	logger.Info("Clearing existing Casbin policies")

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
		}
	}

	logger.Info("Cleared policies")
	return nil
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

	for _, u := range users {
		for _, r := range u.Edges.Roles {
			if _, err := s.manager.AddRoleForUser(u.ID, r.ID); err != nil {
				logger.WithError(err).Warnf("Failed to add user role: user=%s, role=%s", u.ID, r.ID)
				continue
			}
			userRoleCount++
		}
	}

	logger.Infof("Completed user-role sync, processed %d relationships", userRoleCount)
	return nil
}

// syncRolePermissions syncs role permission policies
func (s *SyncService) syncRolePermissions(ctx context.Context) error {
	logger.Info("Starting role permission policy sync")

	// Query all active roles and their menus and API resources
	roles, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.Where().
				WithAPIResources(func(ar *ent.ApiResourceQuery) {
					ar.Where(apiresource.IsPublicEQ(false)) // Only sync API resources that require permission validation
				})
		}).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to query role permission relationships: %w", err)
	}

	policyCount := 0
	for _, r := range roles {
		n, err := s.applyRolePolicies(r.ID, r.Name, r.Edges.Menus)
		if err != nil {
			return err
		}
		policyCount += n
	}

	logger.Infof("Completed role permission policy sync, processed %d policies", policyCount)
	return nil
}

// applyRolePolicies writes a role's permissions to Casbin.
// The admin role receives a wildcard policy; other roles receive policies based on the API resources of their associated menus.
// Returns the number of policies actually written.
func (s *SyncService) applyRolePolicies(roleID, roleName string, menus []*ent.Menu) (int, error) {
	log := logger.WithField("role", roleID)

	if roleName == "admin" {
		if _, err := s.manager.AddPolicy(roleID, "*", "*"); err != nil {
			log.WithError(err).Warn("Failed to add admin wildcard permission")
			return 0, nil
		}
		log.Info("Added wildcard permission for admin role")
		return 1, nil
	}

	count := 0
	for _, menu := range menus {
		for _, apiRes := range menu.Edges.APIResources {
			if _, err := s.manager.AddPolicy(roleID, apiRes.Path, apiRes.Method); err != nil {
				log.WithError(err).Warnf("Failed to add API permission: %s %s", apiRes.Method, apiRes.Path)
				continue
			}
			count++
		}
	}
	return count, nil
}

// SyncUserRole syncs a single user's role relationships
func (s *SyncService) SyncUserRole(ctx context.Context, userID string) error {
	logger.Infof("Syncing user roles: %s", userID)

	// Clear the user's existing roles
	if _, err := s.manager.DeleteRolesForUser(userID); err != nil {
		logger.WithError(err).Warnf("Failed to clear user roles: user=%s", userID)
	}

	// Query the user's current roles (only active users are valid)
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
		return fmt.Errorf("failed to query user roles: %w", err)
	}

	// Re-add user roles
	for _, r := range u.Edges.Roles {
		if _, err := s.manager.AddRoleForUser(userID, r.ID); err != nil {
			logger.WithError(err).Warnf("Failed to add user role: user=%s, role=%s", userID, r.ID)
		}
	}

	return s.manager.SavePolicy()
}

// SyncRolePermissions syncs a single role's permission policies
func (s *SyncService) SyncRolePermissions(ctx context.Context, roleID string) error {
	logger.Infof("Syncing role permissions: %s", roleID)

	// Clear the role's existing permission policies
	if err := s.clearRolePolicies(ctx, roleID); err != nil {
		return fmt.Errorf("failed to clear existing role permissions: %w", err)
	}

	// Query the role's current menus and API resources
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
		return fmt.Errorf("failed to query role permissions: %w", err)
	}

	if _, err := s.applyRolePolicies(roleID, r.Name, r.Edges.Menus); err != nil {
		return err
	}

	return s.manager.SavePolicy()
}

// clearRolePolicies clears all permission policies for a specific role
func (s *SyncService) clearRolePolicies(_ context.Context, roleID string) error {
	if _, err := s.manager.RemoveFilteredPolicy(0, roleID); err != nil {
		logger.WithField("role", roleID).Warnf("Failed to remove role policies: %v", err)
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

// GetSyncStats gets sync statistics
func (s *SyncService) GetSyncStats(ctx context.Context) (*SyncStats, error) {
	stats := &SyncStats{}

	// Count the database's active user-role pairs (not user count)
	users, err := s.entClient.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count user-role relationships: %w", err)
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
		return nil, fmt.Errorf("failed to count role permission policies: %w", err)
	}

	// Count the data in Casbin
	stats.DatabaseUserRoles = userRoleCount
	stats.DatabaseRolePermissions = rolePermCount
	stats.CasbinRoles = len(s.manager.GetAllRoles())
	stats.CasbinPolicies = len(s.manager.GetAllPolicies())

	return stats, nil
}

// SyncStats sync statistics
type SyncStats struct {
	DatabaseUserRoles       int `json:"database_user_roles"`
	DatabaseRolePermissions int `json:"database_role_permissions"`
	CasbinRoles             int `json:"casbin_roles"`
	CasbinPolicies          int `json:"casbin_policies"`
}

// IsHealthy checks whether the sync status is healthy
func (stats *SyncStats) IsHealthy() bool {
	// A simple health check: the data in Casbin should not be empty
	return stats.CasbinRoles > 0 && stats.CasbinPolicies > 0
}
