package cmd

import (
	"fmt"
	"log/slog"

	"shadmin/api"
	bootstrap "shadmin/bootstrap"
	"shadmin/pkg"
)

// 构建时注入的版本信息
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Run 启动应用并返回进程退出码。所有启动失败都在这里统一处理，
// 保证 defer 清理逻辑（关闭数据库、缓存、调度器）总能执行。
func Run() int {
	pkg.InitLogger()
	// 记录版本信息
	slog.Info("starting", "version", version, "commit", commit, "built", date)

	app, err := bootstrap.App()
	if err != nil {
		slog.Error("startup failed", "error", err)
		return 1
	}
	app.Version = version
	defer app.CloseDBConnection()

	if err := start(app); err != nil {
		slog.Error("run failed", "error", err)
		return 1
	}
	return 0
}

func start(app *bootstrap.Application) error {
	// 先设定路由
	if err := api.SetupRoutes(app); err != nil {
		return err
	}

	// 扫描路由入库
	if err := bootstrap.InitApiResources(app); err != nil {
		return fmt.Errorf("API resource initialization failed: %w", err)
	}

	// 初始化默认管理员用户（包含菜单初始化）
	if err := bootstrap.InitDefaultAdmin(app); err != nil {
		return fmt.Errorf("default admin initialization failed: %w", err)
	}

	// 初始化完成后，构建首个 Casbin 授权快照并启动 generation 触发同步。
	if err := bootstrap.InitCasbin(app); err != nil {
		return fmt.Errorf("casbin initialization failed, aborting startup: %w", err)
	}

	return api.Run(app)
}
