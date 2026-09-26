package bootstrap

import (
	"context"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/auth"
	"shadmin/internal/cacher"
	captchapkg "shadmin/internal/captcha"
	"shadmin/internal/casbin"
	"shadmin/internal/conf"
	"shadmin/internal/scheduler"
	"shadmin/repository"
	"time"

	"github.com/gin-gonic/gin"
)

type Application struct {
	Env               *conf.Env
	DB                *ent.Client
	Cacher            cacher.Cacher
	ApiEngine         *gin.Engine
	FileStorage       domain.FileRepository // 新的通用文件存储接口
	CasManager        casbin.Manager
	CasbinInitializer *CasbinInitializer             // Casbin初始化器
	CasbinScheduler   *scheduler.CasbinSyncScheduler // Casbin同步调度器
	CaptchaManager    *captchapkg.SlideManager       // 滑块验证码管理器（内部使用共享 Cacher）
	UserStatusCache   *auth.Cache                    // 用户状态TTL缓存，用于登录/刷新/中间件检查
	TokenBlacklist    auth.JWTBlacklist              // JWT 登出黑名单（内存或 Redis）
	Version           string                         // 应用版本
}

func App() *Application {
	app := &Application{}
	app.Env = conf.NewEnv()
	app.DB = NewEntDatabase(app.Env)

	redisCfg := cacher.RedisConfig{}
	var err error
	if app.Env.RedisEnabled() {
		redisCfg = cacher.RedisConfig{
			Addr:     app.Env.RedisAddress,
			Username: app.Env.RedisUsername,
			Password: app.Env.RedisPassword,
			DB:       app.Env.RedisDB,
		}
	}
	app.Cacher, err = cacher.NewForRuntime(cacher.RuntimeConfig{
		UseRedis: app.Env.RedisEnabled(),
		Redis:    redisCfg,
		Memory:   cacher.MemoryConfig{CleanupInterval: 2 * time.Minute},
	})
	if err != nil {
		panic(err)
	}

	if app.Env.RedisEnabled() {
		log.Printf("Cacher: Redis mode")
	} else {
		log.Printf("Cacher: memory mode")
	}

	// 用户状态缓存：直接复用共享 Cacher，Cache 层做 DB 回源与 TTL 协调。
	app.UserStatusCache = auth.NewUserStatusCacher(
		repository.NewUserRepository(app.DB),
		app.Cacher,
		auth.DefaultTTL,
	)

	// The Ent relationships are authoritative; Casbin policies are in-memory snapshots.
	app.CasManager = casbin.NewCasManager()

	// JWT 登出黑名单：复用共享 Cacher，ns="jwt:blacklist"。
	app.TokenBlacklist = auth.NewTokenBlacklist(app.Cacher)

	// 滑块验证码：复用共享 Cacher，ns="captcha"。
	cm, err := captchapkg.NewSlideManager(app.Cacher)
	if err != nil {
		panic(err)
	}
	app.CaptchaManager = cm

	// 初始化 Casbin 快照构建与同步服务；首个快照在服务启动前构建。
	app.CasbinInitializer = NewCasbinInitializer(app.DB, app.CasManager)

	// Each process refreshes its local snapshot from the shared generation.
	syncService := app.CasbinInitializer.GetSyncService()
	app.CasbinScheduler = scheduler.NewCasbinSyncScheduler(
		syncService,
		time.Duration(app.Env.AuthzSyncPollIntervalSeconds)*time.Second,
	)
	// authz_states generation 提交后立即唤醒当前实例；低频轮询负责兜底恢复。
	app.registerAuthorizationGenerationHook()

	// 初始化文件存储
	storageConfig := InitStorage(app.Env)
	app.FileStorage = storageConfig.FileStorage
	app.ApiEngine = gin.Default()

	// 注册第三方登录 provider（Google/GitHub），并初始化 gothic session store
	InitIdentityProviders(app.Env)

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

	if app.Cacher != nil {
		_ = app.Cacher.Close(context.Background())
	}

	CloseEntConnection(app.DB)
}

// registerAuthorizationGenerationHook 注册 generation 提交触发器。
// 只能在授权事务提交成功后唤醒同步，不能在 generation 更新的事务中直接构建快照。
func (app *Application) registerAuthorizationGenerationHook() {
	if app.CasbinScheduler == nil {
		return
	}

	app.DB.AuthzState.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			mutation, ok := m.(*ent.AuthzStateMutation)
			if !ok || !authzGenerationChanged(mutation) {
				return next.Mutate(ctx, m)
			}

			v, err := next.Mutate(ctx, m)
			if err != nil {
				return nil, err
			}
			afterCommit(m, app.CasbinScheduler.TriggerSync)
			return v, nil
		})
	})
}

func authzGenerationChanged(mutation *ent.AuthzStateMutation) bool {
	if _, ok := mutation.Generation(); ok {
		return true
	}
	_, ok := mutation.AddedGeneration()
	return ok
}

// registerUserStatusCacheHook 注册 ent hook：User 变更提交成功后失效 UserStatusCache。
func (app *Application) registerUserStatusCacheHook() {
	cache := app.UserStatusCache
	app.DB.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if m.Type() != ent.TypeUser {
				return next.Mutate(ctx, m)
			}

			ids := userIDsForCacheInvalidation(ctx, m)

			v, err := next.Mutate(ctx, m)
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				return v, nil
			}

			invalidate := func() {
				for _, id := range ids {
					cache.Invalidate(id)
				}
			}

			afterCommit(m, invalidate)

			return v, nil
		})
	})
}

// userIDsForCacheInvalidation 解析涉及的用户 ID；Create 没有可失效的缓存，直接返回空。
func userIDsForCacheInvalidation(ctx context.Context, m ent.Mutation) []string {
	um, ok := m.(*ent.UserMutation)
	if !ok {
		return nil
	}
	if um.Op().Is(ent.OpCreate) {
		return nil
	}

	ids, err := mutationIDs(ctx, um)
	if err != nil {
		log.WithError(err).Warn("Failed to resolve user IDs for status cache invalidation")
		return nil
	}
	return ids
}
