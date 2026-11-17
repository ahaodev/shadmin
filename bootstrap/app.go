package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"shadmin/api/middleware"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/auth"
	"shadmin/internal/cacher"
	captchapkg "shadmin/internal/captcha"
	"shadmin/internal/casbin"
	"shadmin/internal/conf"
	"shadmin/internal/scheduler"
	"shadmin/internal/tokenservice"
	"shadmin/pkg"
	"shadmin/repository"
	"time"

	"github.com/gin-gonic/gin"
)

type Application struct {
	Env               *conf.Env
	DB                *ent.Client
	Cacher            cacher.Cacher
	ApiEngine         *gin.Engine
	FileStorage       domain.FileRepository
	CasManager        casbin.Manager
	CasbinInitializer *CasbinInitializer
	CasbinScheduler   *scheduler.CasbinSyncScheduler
	CaptchaManager    *captchapkg.SlideManager
	UserStatusCache   *auth.Cache
	TokenBlacklist    domain.TokenBlacklist
	Tokens            domain.TokenIssuer
	Usecases          *Usecases
	Version           string
}

// App 初始化失败时释放已创建的资源，由调用方决定是否退出。
func App() (*Application, error) {
	env, err := conf.NewEnv()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if env.AppEnv == conf.AppEnvDev {
		pkg.SetLogLevel(slog.LevelDebug)
	}
	app := &Application{Env: env}

	app.DB, err = NewEntDatabase(app.Env)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}
	fail := func(format string, args ...any) (*Application, error) {
		app.CloseDBConnection()
		return nil, fmt.Errorf(format, args...)
	}

	redisCfg := cacher.RedisConfig{}
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
		return fail("init cacher: %w", err)
	}

	if app.Env.RedisEnabled() {
		slog.Info("Cacher: Redis mode")
	} else {
		slog.Info("Cacher: memory mode")
	}

	app.UserStatusCache = auth.NewUserStatusCacher(
		repository.NewUserRepository(app.DB),
		app.Cacher,
		auth.DefaultTTL,
	)

	app.CasManager = casbin.NewCasManager()

	app.TokenBlacklist = auth.NewTokenBlacklist(app.Cacher)

	cm, err := captchapkg.NewSlideManager(app.Cacher)
	if err != nil {
		return fail("init captcha: %w", err)
	}
	app.CaptchaManager = cm

	app.CasbinInitializer = NewCasbinInitializer(repository.NewAuthorizationRepository(app.DB), app.CasManager)

	syncService := app.CasbinInitializer.GetSyncService()
	app.CasbinScheduler = scheduler.NewCasbinSyncScheduler(
		syncService,
		time.Duration(app.Env.AuthzSyncPollIntervalSeconds)*time.Second,
	)
	// 提交后唤醒本地同步；轮询兜底遗漏通知。
	app.registerAuthorizationGenerationHook()

	storageConfig, err := InitStorage(app.Env)
	if err != nil {
		return fail("init storage: %w", err)
	}
	app.FileStorage = storageConfig.FileStorage
	if app.Env.AppEnv == conf.AppEnvProd {
		gin.SetMode(gin.ReleaseMode)
	}
	// Gin 访问日志与 panic 日志也写入轮换文件。
	gin.DefaultWriter = pkg.LogWriter
	gin.DefaultErrorWriter = pkg.LogWriter
	app.ApiEngine = gin.New()
	app.ApiEngine.Use(gin.Logger(), middleware.Recovery())

	InitIdentityProviders(app.Env)

	app.registerUserStatusCacheHook()

	app.Tokens = tokenservice.NewTokenService(tokenservice.Config{
		AccessSecret:  app.Env.AccessTokenSecret,
		RefreshSecret: app.Env.RefreshTokenSecret,
		AccessExpiry:  app.Env.AccessTokenExpiryMinute,
		RefreshExpiry: app.Env.RefreshTokenExpiryMinute,
	})
	app.Usecases = newUsecases(app)
	return app, nil
}
func (app *Application) CloseDBConnection() {
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Stop()
	}

	if app.Cacher != nil {
		_ = app.Cacher.Close(context.Background())
	}

	CloseEntConnection(app.DB)
}

// registerAuthorizationGenerationHook 仅在事务提交后触发同步，避免构建未提交的快照。
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

// registerUserStatusCacheHook 在 User 变更提交后失效状态缓存。
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
		slog.Warn("Failed to resolve user IDs for status cache invalidation", "error", err)
		return nil
	}
	return ids
}
