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
				log.Infof("Casbin sync statistics: database user-role relationships: %d, database role-permission relationships: %d, Casbin role mappings: %d, Casbin permission policies: %d",
					stats.DatabaseUserRoles,
					stats.DatabaseRolePermissions,
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

// HealthCheck checks whether Casbin has role mappings and permission policies and is therefore healthy
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

func (t casbinSyncTarget) empty() bool {
	return len(t.userIDs) == 0 &&
		len(t.roleIDs) == 0 &&
		len(t.menuIDs) == 0 &&
		len(t.apiResourceIDs) == 0
}

func (t *casbinSyncTarget) merge(other casbinSyncTarget) {
	t.userIDs = mergeIDs(t.userIDs, other.userIDs)
	t.roleIDs = mergeIDs(t.roleIDs, other.roleIDs)
	t.menuIDs = mergeIDs(t.menuIDs, other.menuIDs)
	t.apiResourceIDs = mergeIDs(t.apiResourceIDs, other.apiResourceIDs)
}

func mergeIDs(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst)+len(src))
	merged := make([]string, 0, len(dst)+len(src))
	for _, id := range append(dst, src...) {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	return merged
}

// triggerHookSync triggers targeted Casbin refresh after permission-related table changes.
// It runs in a background goroutine and waits briefly for transaction propagation.
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
		roleIDs, err := ci.syncService.RoleIDsForMenus(ctx, target.menuIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to query roles associated with menus: %w", err))
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	}

	if len(target.apiResourceIDs) > 0 {
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, target.apiResourceIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to query roles associated with API resources: %w", err))
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
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
	ci.mu.Lock()
	if ci.hooksRegistered {
		ci.mu.Unlock()
		return
	}
	ci.hooksRegistered = true
	ci.mu.Unlock()

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

			if tx := ent.TxFromContext(ctx); tx != nil {
				tx.OnCommit(func(next ent.Committer) ent.Committer {
					return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
						err := next.Commit(ctx, tx)
						if err == nil {
							ci.triggerHookSync(schemaType, target)
						}
						return err
					})
				})
			} else {
				ci.triggerHookSync(schemaType, target)
			}

			return v, nil
		})
	})
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
		roleIDs, err := ci.syncService.RoleIDsForMenus(ctx, ids)
		if err != nil {
			log.WithError(err).Warn("Failed to collect roles associated with menus")
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	case ent.TypeApiResource:
		target.apiResourceIDs = ids
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, ids)
		if err != nil {
			log.WithError(err).Warn("Failed to collect roles associated with API resources")
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	}

	return target
}

func mutationIDs(ctx context.Context, m ent.Mutation) ([]string, error) {
	switch mutation := m.(type) {
	case *ent.UserMutation:
		return idsFromMutation(ctx, mutation.ID, mutation.IDs)
	case *ent.RoleMutation:
		return idsFromMutation(ctx, mutation.ID, mutation.IDs)
	case *ent.MenuMutation:
		return idsFromMutation(ctx, mutation.ID, mutation.IDs)
	case *ent.ApiResourceMutation:
		return idsFromMutation(ctx, mutation.ID, mutation.IDs)
	default:
		return nil, nil
	}
}

func idsFromMutation(ctx context.Context, id func() (string, bool), ids func(context.Context) ([]string, error)) ([]string, error) {
	if singleID, ok := id(); ok {
		return []string{singleID}, nil
	}
	return ids(ctx)
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

// InitCasbinHooks is called after application startup completes: run a full sync (idempotent), register hooks, and start the incremental scheduler
func InitCasbinHooks(app *Application) {
	ctx := context.Background()
	if err := app.CasbinInitializer.InitializeCasbin(ctx); err != nil {
		log.WithError(err).Error("initial casbin sync failed")
	}
	app.CasbinInitializer.SetupHooks()
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Start(ctx)
	}
}
