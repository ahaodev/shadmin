package route

import (
	"time"

	"shadmin/api/middleware"
	"shadmin/bootstrap"
	"shadmin/internal/conf"

	"github.com/gin-gonic/gin"
)

type PublicRoutes struct {
	factory *ControllerFactory
}

func NewPublicRoutes(factory *ControllerFactory) *PublicRoutes {
	return &PublicRoutes{
		factory: factory,
	}
}

func (pr *PublicRoutes) Setup(router *gin.RouterGroup, app *bootstrap.Application) {
	if app.Env.AppEnv == conf.AppEnvDev {
		router.Use(middleware.LogMiddleware())
	}

	healthGroup := router.Group("/health")
	pr.setupHealthRoutes(healthGroup)

	authGroup := router.Group("/auth")
	pr.setupAuthRoutes(authGroup)
}

func (pr *PublicRoutes) setupHealthRoutes(group *gin.RouterGroup) {
	hc := pr.factory.CreateHealthController()
	group.GET("", hc.Health)
}

func (pr *PublicRoutes) setupAuthRoutes(group *gin.RouterGroup) {
	authController := pr.factory.CreateAuthController()
	captchaController := pr.factory.CreateCaptchaController()
	deviceAuthController := pr.factory.CreateDeviceAuthController()
	userIdentityController := pr.factory.CreateUserIdentityController()
	userInvitationController := pr.factory.CreateUserInvitationController()

	group.POST("/login", authController.Login)
	group.POST("/refresh", authController.RefreshToken)
	group.POST("/logout", authController.Logout)
	group.GET("/captcha/slide", captchaController.GetSlideCaptcha)
	group.POST("/device/code", middleware.RateLimitByIP(10, time.Minute), deviceAuthController.RequestCode)
	group.POST("/device/token", middleware.RateLimitByIP(60, time.Minute), deviceAuthController.PollToken)
	group.POST("/invitations/accept", middleware.RateLimitByIP(10, time.Minute), userInvitationController.Accept)

	identity := group.Group("/identity")
	identity.GET("/providers", userIdentityController.ListProviders)
	identity.POST("/exchange", userIdentityController.Exchange)
	identity.GET("/:provider", userIdentityController.BeginLogin)
	identity.GET("/:provider/callback", userIdentityController.Callback)
}
