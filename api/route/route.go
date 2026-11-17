package route

import (
	"shadmin/bootstrap"
	"shadmin/frontend"
	"shadmin/internal/conf"

	"github.com/gin-gonic/gin"
)

const ApiUri = "/api/v1"

func Setup(app *bootstrap.Application, engine *gin.Engine) error {
	config := DefaultServerConfig()
	if err := config.Apply(engine); err != nil {
		return err
	}

	if app.Env.AppEnv != conf.AppEnvDev {
		frontend.Register(engine)
	}
	setupSwagger(engine)

	setupApiRoutes(app, engine)

	return nil
}

func setupApiRoutes(app *bootstrap.Application, engine *gin.Engine) {
	apiV1 := engine.Group(ApiUri)
	factory := NewControllerFactory(app)

	publicRoutes := NewPublicRoutes(factory)
	publicRoutes.Setup(apiV1, app)

	protectedRoutes := NewProtectedRoutes(factory)
	protectedRoutes.Setup(apiV1, app)
}
