package route

import (
	"shadmin/api/middleware"
	"shadmin/bootstrap"
	"shadmin/internal/conf"

	"github.com/gin-gonic/gin"
)

// PublicRoutes manages all public (unauthenticated) routes
type PublicRoutes struct {
	factory *ControllerFactory
}

// NewPublicRoutes creates a new public routes manager
func NewPublicRoutes(factory *ControllerFactory) *PublicRoutes {
	return &PublicRoutes{
		factory: factory,
	}
}

// Setup configures all public routes
func (pr *PublicRoutes) Setup(router *gin.RouterGroup, app *bootstrap.Application) {
	if app.Env.AppEnv == conf.AppEnvDev {
		router.Use(middleware.LogMiddleware())
	}

	// Health check
	healthGroup := router.Group("/health")
	pr.setupHealthRoutes(healthGroup)

	// Authentication routes
	authGroup := router.Group("/auth")
	pr.setupAuthRoutes(authGroup, app)
}

// setupHealthRoutes configures health check routes
func (pr *PublicRoutes) setupHealthRoutes(group *gin.RouterGroup) {
	hc := pr.factory.CreateHealthController()
	group.GET("", hc.Health)
}

// setupAuthRoutes configures authentication-related routes
func (pr *PublicRoutes) setupAuthRoutes(group *gin.RouterGroup, app *bootstrap.Application) {
	authController := pr.factory.CreateAuthController(app.CasManager)
	captchaController := pr.factory.CreateCaptchaController()
	deviceAuthController := pr.factory.CreateDeviceAuthController()

	group.POST("/login", authController.Login)
	group.POST("/refresh", authController.RefreshToken)
	group.POST("/logout", authController.Logout)
	group.GET("/captcha/slide", captchaController.GetSlideCaptcha)
	group.POST("/device/code", deviceAuthController.RequestCode)
	group.POST("/device/token", deviceAuthController.PollToken)
}
