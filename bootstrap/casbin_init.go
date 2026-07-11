package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"shadmin/ent"
	"shadmin/internal/casbin"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/logger"
)

// CasbinInitializer casbin初始化器
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

// NewCasbinInitializer 创建casbin初始化器
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

// InitializeCasbin 初始化casbin系统
// 这个方法会在应用启动时调用，执行以下操作：
// 1. 初始化casbin管理器
// 2. 从数据库同步权限数据到casbin
// 3. 验证同步结果
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

	logger.Info("Casbin Initializing...")

	startTime := time.Now()
	var err error

	// 1. 验证casbin管理器是否已初始化
	if ci.casManager == nil {
		err = fmt.Errorf("casbin管理器未初始化")
	} else {
		// 2. 从数据库同步权限数据到casbin
		log.Info("DB to Casbin ...")
		if syncErr := ci.syncService.SyncFromDatabase(ctx); syncErr != nil {
			err = fmt.Errorf("从数据库同步权限数据失败: %w", syncErr)
		} else {
			// 3. 验证同步结果
			stats, statsErr := ci.syncService.GetSyncStats(ctx)
			if statsErr != nil {
				log.WithError(statsErr).Warn("获取同步统计失败")
			} else {
				log.Infof("Casbin同步统计: 数据库用户角色关系: %d, 数据库角色权限关系: %d, Casbin角色映射: %d, Casbin权限策略: %d",
					stats.DatabaseUserRoles,
					stats.DatabaseRolePermissions,
					stats.CasbinRoles,
					stats.CasbinPolicies)

				if !stats.IsHealthy() {
					log.Warn("Casbin同步状态可能不健康，请检查数据")
				}
			}
		}
	}

	duration := time.Since(startTime)
	if err != nil {
		log.WithError(err).Errorf("Casbin权限系统初始化失败，耗时: %v", duration)
	} else {
		log.Infof("Casbin权限系统初始化完成，耗时: %v", duration)
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

// InitError 返回首次初始化同步的错误（nil 表示成功或尚未初始化）
func (ci *CasbinInitializer) InitError() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.initErr
}

// SyncFromDatabase 手动触发从数据库全量同步（绕过初始化状态检查）
func (ci *CasbinInitializer) SyncFromDatabase(ctx context.Context) error {
	return ci.syncService.SyncFromDatabase(ctx)
}

// GetSyncStats 获取同步统计信息
func (ci *CasbinInitializer) GetSyncStats(ctx context.Context) (*casbin.SyncStats, error) {
	return ci.syncService.GetSyncStats(ctx)
}

// GetSyncService 获取同步服务实例
func (ci *CasbinInitializer) GetSyncService() *casbin.SyncService {
	return ci.syncService
}

// HealthCheck 健康检查：casbin 中有角色映射且有权限策略则视为健康
func (ci *CasbinInitializer) HealthCheck(ctx context.Context) (bool, error) {
	stats, err := ci.syncService.GetSyncStats(ctx)
	if err != nil {
		return false, fmt.Errorf("获取同步统计失败: %w", err)
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

// triggerHookSync 在权限相关表变更后，按变更对象定向刷新 Casbin。
// 在后台 goroutine 中执行，稍作延迟以等待事务传播。
func (ci *CasbinInitializer) triggerHookSync(schemaType string, target casbinSyncTarget) {
	if target.empty() {
		log.Debugf("跳过 Casbin 定向同步 (来源: %s 变更，无目标对象)", schemaType)
		return
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		syncCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		log.Infof("触发 Casbin 定向同步 (来源: %s 变更, users=%d, roles=%d, menus=%d, api_resources=%d)",
			schemaType, len(target.userIDs), len(target.roleIDs), len(target.menuIDs), len(target.apiResourceIDs))
		if err := ci.syncTarget(syncCtx, target); err != nil {
			log.WithError(err).Error("Casbin 定向同步失败")
		}
	}()
}

func (ci *CasbinInitializer) syncTarget(ctx context.Context, target casbinSyncTarget) error {
	var errs []error

	for _, userID := range target.userIDs {
		if err := ci.syncService.SyncUserRole(ctx, userID); err != nil {
			log.WithError(err).Warnf("同步用户角色失败: user=%s", userID)
			errs = append(errs, fmt.Errorf("同步用户 %s 失败: %w", userID, err))
		}
	}

	if len(target.menuIDs) > 0 {
		roleIDs, err := ci.syncService.RoleIDsForMenus(ctx, target.menuIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("查询菜单关联角色失败: %w", err))
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	}

	if len(target.apiResourceIDs) > 0 {
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, target.apiResourceIDs)
		if err != nil {
			errs = append(errs, fmt.Errorf("查询API资源关联角色失败: %w", err))
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	}

	for _, roleID := range target.roleIDs {
		if err := ci.syncService.SyncRolePermissions(ctx, roleID); err != nil {
			log.WithError(err).Warnf("同步角色权限失败: role=%s", roleID)
			errs = append(errs, fmt.Errorf("同步角色 %s 失败: %w", roleID, err))
		}
	}

	return errors.Join(errs...)
}

// SetupHooks 注册 Ent Hook，在 User/Role/Menu/APIResource 变更提交后触发 Casbin 定向同步
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
		log.WithError(err).Warnf("收集 Casbin 同步目标失败 (schema=%s)", m.Type())
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
			log.WithError(err).Warn("收集菜单关联角色失败")
		} else {
			target.roleIDs = mergeIDs(target.roleIDs, roleIDs)
		}
	case ent.TypeApiResource:
		target.apiResourceIDs = ids
		roleIDs, err := ci.syncService.RoleIDsForAPIResources(ctx, ids)
		if err != nil {
			log.WithError(err).Warn("收集API资源关联角色失败")
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

// InitCasbinHooks 在应用启动完成后调用：执行一次全量同步（幂等）、注册 Hook 并启动增量调度器
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
