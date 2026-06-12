package bootstrap

import (
	"context"
	"log"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/casbin"
	"shadmin/internal/scheduler"
	"shadmin/internal/userstatus"
	"shadmin/repository"
	"time"

	"github.com/gin-gonic/gin"
)

type Application struct {
	Env               *Env
	DB                *ent.Client
	ApiEngine         *gin.Engine
	FileStorage       domain.FileRepository // 新的通用文件存储接口
	CasManager        casbin.Manager
	CasbinInitializer *CasbinInitializer             // Casbin初始化器
	CasbinScheduler   *scheduler.CasbinSyncScheduler // Casbin同步调度器
	UserStatusCache   *userstatus.Cache              // 用户状态TTL缓存，用于登录/刷新/中间件检查
	Version           string                         // 应用版本
}

func App() *Application {
	app := &Application{}
	app.Env = NewEnv()
	app.DB = NewEntDatabase(app.Env)

	// 初始化全局Casbin管理器
	if err := casbin.Initialize(app.DB); err != nil {
		panic(err)
	}
	app.CasManager = casbin.GetManager()

	// 初始化用户状态缓存
	app.UserStatusCache = userstatus.New(
		repository.NewUserRepository(app.DB, app.CasManager),
		userstatus.DefaultTTL,
	)

	// 初始化Casbin初始化器并执行启动时同步
	app.CasbinInitializer = NewCasbinInitializer(app.DB, app.CasManager)

	// 执行启动时的casbin同步
	ctx := context.Background()
	if err := app.CasbinInitializer.InitializeCasbin(ctx); err != nil {
		log.Printf("ERROR: Casbin初始化失败: %v", err)
		// 不panic，允许应用继续启动，但会影响权限功能
	}

	// 初始化并启动Casbin定时同步调度器（每1小时同步一次作为兜底）
	syncService := app.CasbinInitializer.GetSyncService()
	app.CasbinScheduler = scheduler.NewCasbinSyncScheduler(syncService, 1*time.Hour)
	app.CasbinScheduler.Start(ctx)

	// 初始化文件存储
	storageConfig := InitStorage(app.Env)
	app.FileStorage = storageConfig.FileStorage
	app.ApiEngine = gin.Default()

	// 注册 User ent hook：状态变更时让缓存失效，
	// 保证 admin 禁用/启用/邀请/恢复用户后，下一次请求即可看到新状态。
	app.registerUserStatusCacheHook()

	return app
}
func (app *Application) CloseDBConnection() {
	// 停止Casbin同步调度器
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Stop()
	}

	CloseEntConnection(app.DB)
}

// registerUserStatusCacheHook 在 User 表的 UpdateOne / Update / Delete 上注册
// 一个 ent hook，变更提交后调用 UserStatusCache.Invalidate(id)。
func (app *Application) registerUserStatusCacheHook() {
	cache := app.UserStatusCache
	app.DB.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			v, err := next.Mutate(ctx, m)
			if err != nil {
				return nil, err
			}
			if m.Type() != ent.TypeUser {
				return v, nil
			}

			// ent.Mutation 接口没有 ID()，需要用具体类型拿到目标用户 ID。
			um, ok := m.(*ent.UserMutation)
			if !ok {
				return v, nil
			}
			id, idExists := um.ID()

			invalidate := func() {
				if idExists && id != "" {
					cache.Invalidate(id)
				}
			}

			if tx := ent.TxFromContext(ctx); tx != nil {
				tx.OnCommit(func(next ent.Committer) ent.Committer {
					return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
						err := next.Commit(ctx, tx)
						if err == nil {
							invalidate()
						}
						return err
					})
				})
			} else {
				invalidate()
			}

			return v, nil
		})
	})
}
