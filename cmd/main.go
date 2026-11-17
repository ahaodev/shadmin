package cmd

import (
	"flag"
	"fmt"
	"log/slog"
	"shadmin/internal/conf"

	"shadmin/api"
	"shadmin/bootstrap"
	"shadmin/pkg"
)

// 构建时注入的版本信息
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Run 返回退出码，确保启动失败时仍执行 defer 清理。
func Run() int {
	migrate := flag.Bool("migrate", false, "apply additive schema migration and exit (back up first)")
	flag.Parse()
	pkg.InitLogger()
	if *migrate {
		env, err := conf.NewEnv()
		if err == nil {
			err = bootstrap.MigrateDatabase(env)
		}
		if err != nil {
			slog.Error("migration failed", "error", err)
			return 1
		}
		return 0
	}
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
	if err := api.SetupRoutes(app); err != nil {
		return err
	}

	if err := bootstrap.InitApiResources(app); err != nil {
		return fmt.Errorf("API resource initialization failed: %w", err)
	}

	if err := bootstrap.InitDefaultAdmin(app); err != nil {
		return fmt.Errorf("default admin initialization failed: %w", err)
	}

	if err := bootstrap.InitCasbin(app); err != nil {
		return fmt.Errorf("casbin initialization failed, aborting startup: %w", err)
	}

	return api.Run(app)
}
