package bootstrap

import (
	"context"
	"log"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/auth/tokenblacklist"
	captchapkg "shadmin/internal/captcha"
	"shadmin/internal/cache"
	"shadmin/internal/casbin"
	"shadmin/internal/cachex"
	"shadmin/internal/scheduler"
	"shadmin/internal/userstatus"
	"shadmin/repository"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Application struct {
	Env               *Env
	DB                *ent.Client
	Redis             *redis.Client // 启用 Redis 时非 nil（captcha/jwt/userstatus 用）；casbin 适配器自行管理连接
	Cacher            cachex.Cacher // 统一缓存后端：REDIS_ADDR 非空用 Redis，否则进程内 go-cache。供 tokenblacklist/captcha/userstatus 共用
	ApiEngine         *gin.Engine
	FileStorage       domain.FileRepository // 新的通用文件存储接口
	CasManager        casbin.Manager
	CasbinInitializer *CasbinInitializer             // Casbin初始化器
	CasbinScheduler   *scheduler.CasbinSyncScheduler // Casbin同步调度器
	CaptchaManager    *captchapkg.SlideManager       // 滑块验证码管理器（内部使用共享 Cacher）
	UserStatusCache   *userstatus.Cache              // 用户状态TTL缓存，用于登录/刷新/中间件检查
	TokenBlacklist    tokenblacklist.Blacklist       // JWT 登出黑名单（内存或 Redis）
	Version           string                         // 应用版本
}

func App() *Application {
	app := &Application{}
	app.Env = NewEnv()
	app.DB = NewEntDatabase(app.Env)

	// 可选：初始化 Redis 客户端（REDIS_ADDR 非空时启用）
	if app.Env.RedisEnabled() {
		cli, err := cache.NewClient(app.Env.RedisAddr, app.Env.RedisPassword, app.Env.RedisDB)
		if err != nil {
			panic(err)
		}
		app.Redis = cli
		log.Printf("Redis 已启用: %s db=%d", app.Env.RedisAddr, app.Env.RedisDB)
	}

	// 初始化全局Casbin管理器（启用 Redis 时走 redis-adapter，否则内存模式）
	if err := casbin.Initialize(app.DB, casbin.Config{
		RedisAddr:     app.Env.RedisAddr,
		RedisPassword: app.Env.RedisPassword,
		RedisDB:       app.Env.RedisDB,
	}); err != nil {
		panic(err)
	}
	app.CasManager = casbin.GetManager()

	// 初始化统一缓存后端：REDIS_ADDR 非空 → Redis cacher（共用上方 app.Redis 客户端），
	// 否则进程内 go-cache。tokenblacklist / captcha / userstatus 三个模块共用同一实例，
	// 通过各自的 namespace 隔离。
	if app.Redis != nil {
		app.Cacher = cachex.NewRedisCacheWithClient(app.Redis)
		log.Printf("Cacher: Redis mode")
	} else {
		app.Cacher = cachex.NewMemoryCache(cachex.MemoryConfig{CleanupInterval: 2 * time.Minute})
		log.Printf("Cacher: memory mode")
	}

	// 用户状态缓存：直接复用共享 Cacher，Cache 层做 DB 回源与 TTL 协调。
	app.UserStatusCache = userstatus.New(
		repository.NewUserRepository(app.DB, app.CasManager),
		userstatus.NewStore(app.Cacher),
		userstatus.DefaultTTL,
	)

	// JWT 登出黑名单：复用共享 Cacher，ns="jwt:blacklist"。
	app.TokenBlacklist = tokenblacklist.New(app.Cacher)

	// 滑块验证码：复用共享 Cacher，ns="captcha"。
	cm, err := captchapkg.NewSlideManager(captchapkg.NewStore(app.Cacher))
	if err != nil {
		panic(err)
	}
	app.CaptchaManager = cm

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

	// Cacher 由 bootstrap 自建 Redis 客户端时负责关闭；此处复用 app.Redis，
	// Close 是 no-op，真正的 Redis 连接在下方统一关闭。
	if app.Cacher != nil {
		_ = app.Cacher.Close(context.Background())
	}

	if app.Redis != nil {
		_ = app.Redis.Close()
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
