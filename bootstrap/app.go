package bootstrap

import (
	"context"
	"log"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/casbin"
	"shadmin/internal/scheduler"
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
	return app
}
func (app *Application) CloseDBConnection() {
	// 停止Casbin同步调度器
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Stop()
	}

	CloseEntConnection(app.DB)
}
