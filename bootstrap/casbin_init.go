package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"shadmin/ent"
	"shadmin/internal/casbin"
	"sync"
	"time"
)

// CasbinInitializer casbin initializer
type CasbinInitializer struct {
	entClient       *ent.Client
	syncService     *casbin.SyncService
	casManager      casbin.Manager
	initialized     bool
	initializing    bool
	initErr         error
	hooksRegistered bool
	mu              sync.Mutex
	cond            *sync.Cond
}

// NewCasbinInitializer creates a casbin initializer
func NewCasbinInitializer(entClient *ent.Client, casManager casbin.Manager) *CasbinInitializer {
	syncService := casbin.NewSyncService(entClient, casManager)
	ci := &CasbinInitializer{
		entClient:   entClient,
		syncService: syncService,
		casManager:  casManager,
	}
	ci.cond = sync.NewCond(&ci.mu)
	return ci
}

// InitializeCasbin initializes the casbin system.
// This method is called during application startup and performs the following:
// 1. Initializes the casbin manager
// 2. Synchronizes permission data from the database to casbin
// 3. Validates the sync result
func (ci *CasbinInitializer) InitializeCasbin(ctx context.Context) error {
	ci.mu.Lock()
	for ci.initializing {
		ci.cond.Wait()
	}
	if ci.initialized {
		ci.mu.Unlock()
		return nil
	}

	ci.initializing = true
	ci.mu.Unlock()

	log.Info("Casbin initializing...")

	startTime := time.Now()
	var err error

	// 1. Validate that the casbin manager is initialized
	if ci.casManager == nil {
		err = fmt.Errorf("casbin manager is not initialized")
	} else {
		// 2. Synchronize permission data from the database to casbin
		log.Info("Syncing database data to Casbin...")
		if syncErr := ci.syncService.SyncFromDatabase(ctx); syncErr != nil {
			err = fmt.Errorf("failed to sync permission data from the database: %w", syncErr)
		} else {
			// 3. Validate the synchronization result
			stats, statsErr := ci.syncService.GetSyncStats(ctx)
			if statsErr != nil {
				log.WithError(statsErr).Warn("failed to get sync statistics")
			} else {
				log.Infof("Casbin sync statistics: database user-role relationships: %d, database active roles: %d, Casbin role mappings: %d, Casbin permission policies: %d",
					stats.DatabaseUserRoles,
					stats.DatabaseRoles,
					stats.CasbinRoles,
					stats.CasbinPolicies)

				if !stats.IsHealthy() {
					log.Warn("Casbin sync state may be unhealthy; please check the data")
				}
			}
		}
	}

	duration := time.Since(startTime)
	if err != nil {
		log.WithError(err).Errorf("Casbin permission system initialization failed, elapsed: %v", duration)
	} else {
		log.Infof("Casbin permission system initialization completed, elapsed: %v", duration)
	}

	ci.mu.Lock()
	ci.initErr = err
	if err == nil {
		ci.initialized = true
	}
	ci.initializing = false
	ci.cond.Broadcast()
	ci.mu.Unlock()

	return err
}

// InitError returns the first initialization sync error (nil means success or not initialized yet)
func (ci *CasbinInitializer) InitError() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.initErr
}

// SyncFromDatabase manually triggers a full sync from the database (bypassing initialization state checks)
func (ci *CasbinInitializer) SyncFromDatabase(ctx context.Context) error {
	return ci.syncService.SyncFromDatabase(ctx)
}

// GetSyncStats gets sync statistics
func (ci *CasbinInitializer) GetSyncStats(ctx context.Context) (*casbin.SyncStats, error) {
	return ci.syncService.GetSyncStats(ctx)
}

// GetSyncService gets the sync service instance
func (ci *CasbinInitializer) GetSyncService() *casbin.SyncService {
	return ci.syncService
}

// HealthCheck reports whether the synced state is consistent: when the database holds no
// user-role or role rows, Casbin must hold no residual roles or policies.
func (ci *CasbinInitializer) HealthCheck(ctx context.Context) (bool, error) {
	stats, err := ci.syncService.GetSyncStats(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get sync statistics: %w", err)
	}
	return stats.IsHealthy(), nil
}

type casbinSyncTarget struct {
	userIDs        []string
	roleIDs        []string
	menuIDs        []string
	apiResourceIDs []string
}

func (t *casbinSyncTarget) empty() bool {
	return len(t.userIDs) == 0 &&
		len(t.roleIDs) == 0 &&
		len(t.menuIDs) == 0 &&
		len(t.apiResourceIDs) == 0
}

func (t *casbinSyncTarget) merge(other casbinSyncTarget) {
	t.userIDs = casbin.MergeUniqueIDs(t.userIDs, other.userIDs)
	t.roleIDs = casbin.MergeUniqueIDs(t.roleIDs, other.roleIDs)
	t.menuIDs = casbin.MergeUniqueIDs(t.menuIDs, other.menuIDs)
	t.apiResourceIDs = casbin.MergeUniqueIDs(t.apiResourceIDs, other.apiResourceIDs)
}

// triggerHookSync triggers targeted Casbin refresh after permission-related table changes.
// 后台同步带 100ms 延迟，等事务提交完成；Casbin 侧写入由 SyncService 的锁串行化，
// 因此并发触发只是排队，不会互相覆盖。
func (ci *CasbinInitializer) triggerHookSync(schemaType string, target casbinSyncTarget) {
	if target.empty() {
		log.Debugf("Skipping targeted Casbin sync (source: %s change, no target objects)", schemaType)
		return
	}

	go func() {
		time.Sleep(100 * time.Millisecond)

		syncCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		log.Infof("Triggering targeted Casbin sync (source: %s change, users=%d, roles=%d, menus=%d, api_resources=%d)",
			schemaType, len(target.userIDs), len(target.roleIDs), len(target.menuIDs), len(target.apiResourceIDs))
		if err := ci.syncTarget(syncCtx, target); err != nil {
			log.WithError(err).Error("Targeted Casbin sync failed")
		}
	}()
}

func (ci *CasbinInitializer) syncTarget(ctx context.Context, target casbinSyncTarget) error {
	var errs []error

	for _, userID := range target.userIDs {
		if err := ci.syncService.SyncUserRole(ctx, userID); err != nil {
			log.WithError(err).Warnf("Failed to sync user roles: user=%s", userID)
			errs = append(errs, fmt.Errorf("failed to sync user %s: %w", userID, err))
		}
	}

	if len(target.menuIDs) > 0 {
		// 变更后再查一次角色：关联可能被"扩大"（如给 menu 新增 api_resource），
		// 变更前的快照里没有这些角色。
		roleIDs, err := ci.syncService.RoleIDsForMenus(ctx, target.menuIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to query roles associated with menus: %w", err))
		} else {
			target.roleIDs = casbin.MergeUniqueIDs(target.roleIDs, roleIDs)
		}
	}

	if len(target.apiResourceIDs) > 0 {
		// 同上。
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, target.apiResourceIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to query roles associated with API resources: %w", err))
		} else {
			target.roleIDs = casbin.MergeUniqueIDs(target.roleIDs, roleIDs)
		}
	}

	for _, roleID := range target.roleIDs {
		if err := ci.syncService.SyncRolePermissions(ctx, roleID); err != nil {
			log.WithError(err).Warnf("Failed to sync role permissions: role=%s", roleID)
			errs = append(errs, fmt.Errorf("failed to sync role %s: %w", roleID, err))
		}
	}

	return errors.Join(errs...)
}

// SetupHooks registers Ent hooks so that User/Role/Menu/APIResource changes trigger targeted Casbin sync after commit
func (ci *CasbinInitializer) SetupHooks() {
	if !ci.markHooksRegistered() {
		return
	}

	ci.entClient.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			schemaType := m.Type()
			if schemaType != ent.TypeUser && schemaType != ent.TypeRole && schemaType != ent.TypeMenu && schemaType != ent.TypeApiResource {
				return next.Mutate(ctx, m)
			}

			target := ci.collectHookTarget(ctx, m)

			v, err := next.Mutate(ctx, m)
			if err != nil {
				return nil, err
			}
			target.merge(ci.collectValueTarget(v))

			afterCommit(m, func() {
				ci.triggerHookSync(schemaType, target)
			})

			return v, nil
		})
	})
}

// markHooksRegistered 标记 hook 已注册；返回 false 表示非首次。
func (ci *CasbinInitializer) markHooksRegistered() bool {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if ci.hooksRegistered {
		return false
	}
	ci.hooksRegistered = true
	return true
}

func (ci *CasbinInitializer) collectHookTarget(ctx context.Context, m ent.Mutation) casbinSyncTarget {
	target := casbinSyncTarget{}
	ids, err := mutationIDs(ctx, m)
	if err != nil {
		log.WithError(err).Warnf("Failed to collect Casbin sync targets (schema=%s)", m.Type())
		return target
	}

	switch m.Type() {
	case ent.TypeUser:
		target.userIDs = ids
	case ent.TypeRole:
		target.roleIDs = ids
	case ent.TypeMenu:
		target.menuIDs = ids
		// 变更前解析关联角色：变更可能“收缩”关联（解除 menu↔role 或级联删除），
		// 生效后再查就找不到这些角色了，而它们正是需要被回收权限的对象。
		roleIDs, err := ci.syncService.RoleIDsForMenus(ctx, ids)
		if err != nil {
			log.WithError(err).Warn("Failed to collect roles associated with menus")
		} else {
			target.roleIDs = casbin.MergeUniqueIDs(target.roleIDs, roleIDs)
		}
	case ent.TypeApiResource:
		target.apiResourceIDs = ids
		// 同上：级联删除后关联关系不可再查，必须在变更前取快照。
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, ids)
		if err != nil {
			log.WithError(err).Warn("Failed to collect roles associated with API resources")
		} else {
			target.roleIDs = casbin.MergeUniqueIDs(target.roleIDs, roleIDs)
		}
	}

	return target
}

func (ci *CasbinInitializer) collectValueTarget(v ent.Value) casbinSyncTarget {
	switch value := v.(type) {
	case *ent.User:
		return casbinSyncTarget{userIDs: []string{value.ID}}
	case []*ent.User:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			ids = append(ids, item.ID)
		}
		return casbinSyncTarget{userIDs: ids}
	case *ent.Role:
		return casbinSyncTarget{roleIDs: []string{value.ID}}
	case []*ent.Role:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			ids = append(ids, item.ID)
		}
		return casbinSyncTarget{roleIDs: ids}
	case *ent.Menu:
		return casbinSyncTarget{menuIDs: []string{value.ID}}
	case []*ent.Menu:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			ids = append(ids, item.ID)
		}
		return casbinSyncTarget{menuIDs: ids}
	case *ent.ApiResource:
		return casbinSyncTarget{apiResourceIDs: []string{value.ID}}
	case []*ent.ApiResource:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			ids = append(ids, item.ID)
		}
		return casbinSyncTarget{apiResourceIDs: ids}
	default:
		return casbinSyncTarget{}
	}
}

// InitCasbinHooks is called after application startup completes: run a full sync (idempotent), register hooks, and start the incremental scheduler.
func InitCasbinHooks(app *Application) error {
	ctx := context.Background()
	if err := app.CasbinInitializer.InitializeCasbin(ctx); err != nil {
		return fmt.Errorf("initial casbin sync failed: %w", err)
	}
	app.CasbinInitializer.SetupHooks()
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Start(ctx)
	}
	return nil
}
