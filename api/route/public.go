package route

import (
	"shadmin/api/middleware"
	bootstrap "shadmin/bootstrap"
	"time"

	"github.com/gin-gonic/gin"
)

// PublicRoutes manages all public (unauthenticated) routes
type PublicRoutes struct {
	factory *ControllerFactory
}

// NewPublicRoutes creates a new public routes manager
func NewPublicRoutes(app *bootstrap.Application, timeout time.Duration) *PublicRoutes {
	return &PublicRoutes{
		factory: NewControllerFactory(app, timeout, app.DB),
	}
}

// Setup configures all public routes
func (pr *PublicRoutes) Setup(router *gin.RouterGroup, app *bootstrap.Application) {
	// Add development logging middleware
	if app.Env.AppEnv == "development" {
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

	group.POST("/login", authController.Login)
	group.POST("/refresh", authController.RefreshToken)
	group.POST("/logout", authController.Logout)
}
