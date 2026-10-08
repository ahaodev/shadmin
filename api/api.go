package api

import (
	"shadmin/api/route"
	"shadmin/bootstrap"
	"shadmin/pkg"
)

// SetupRoutes 设置所有路由
func SetupRoutes(app *bootstrap.Application) {
	// 设置路由
	if err := route.Setup(app, app.ApiEngine); err != nil {
		pkg.Log.Printf("Failed to setup routes: %v", err)
	}
}

func Run(app *bootstrap.Application) error {
	// 启动服务器
	port := []string{app.Env.Port}
	err := app.ApiEngine.Run(port...)
	return err
}
