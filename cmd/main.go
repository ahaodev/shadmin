package cmd

import (
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

func Run() {
	// 记录版本信息
	pkg.Log.Infof("starting - Version: %s, Commit: %s, Built: %s", version, commit, date)

	app := bootstrap.App()
	app.Version = version
	defer app.CloseDBConnection()

	// 先设定路由
	api.SetupRoutes(app)

	// 扫描路由入库
	if err := bootstrap.InitApiResources(app); err != nil {
		pkg.Log.Fatalf("API resource initialization failed: %v", err)
	}

	// 初始化默认管理员用户（包含菜单初始化）
	if err := bootstrap.InitDefaultAdmin(app); err != nil {
		pkg.Log.Fatalf("default admin initialization failed: %v", err)
	}

	// 初始化完成后，构建首个 Casbin 授权快照并启动 generation 触发同步。
	if err := bootstrap.InitCasbin(app); err != nil {
		pkg.Log.Fatalf("casbin initialization failed, aborting startup: %v", err)
	}

	err := api.Run(app)
	if err != nil {
		pkg.Log.Error(err)
	}
}
