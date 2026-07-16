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
	socialAuthController := pr.factory.CreateSocialAuthController()

	group.POST("/login", authController.Login)
	group.POST("/refresh", authController.RefreshToken)
	group.POST("/logout", authController.Logout)
	group.GET("/captcha/slide", captchaController.GetSlideCaptcha)
	group.POST("/device/code", deviceAuthController.RequestCode)
	group.POST("/device/token", deviceAuthController.PollToken)

	// 第三方登录：先注册 /providers，再注册 /:provider 与 /:provider/callback，
	// 避免 gin 路由树把 /providers 当作 :provider 匹配。
	social := group.Group("/social")
	social.GET("/providers", socialAuthController.ListProviders)
	social.POST("/exchange", socialAuthController.Exchange)
	social.GET("/:provider", socialAuthController.BeginLogin)
	social.GET("/:provider/callback", socialAuthController.Callback)
}
