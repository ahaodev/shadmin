package route

import (
	"shadmin/api/middleware"
	"shadmin/bootstrap"

	"github.com/gin-gonic/gin"
)

// ProtectedRoutes manages all authenticated routes
type ProtectedRoutes struct {
	factory *ControllerFactory
}

// NewProtectedRoutes creates a new protected routes manager
func NewProtectedRoutes(factory *ControllerFactory) *ProtectedRoutes {
	return &ProtectedRoutes{
		factory: factory,
	}
}

// Setup configures all protected routes with authentication middleware
func (pr *ProtectedRoutes) Setup(router *gin.RouterGroup, app *bootstrap.Application, engine *gin.Engine) {
	// Apply authentication middleware first, then enforce token state checks.
	protectedRouter := router.Group("")
	protectedRouter.Use(middleware.JwtAuthMiddleware(app.Env.AccessTokenSecret, app.TokenBlacklist))
	protectedRouter.Use(middleware.UserStateMiddleware(app.UserStatusCache))

	// Setup different route groups
	pr.setupUserRoutes(protectedRouter, app)
	pr.setupSystemRoutes(protectedRouter, app, engine)
}

// setupUserRoutes configures basic user functionality routes
func (pr *ProtectedRoutes) setupUserRoutes(router *gin.RouterGroup, app *bootstrap.Application) {
	// Profile management
	profileGroup := router.Group("/profile")
	pr.setupProfileRoutes(profileGroup)

	// Resource menu access
	resourcesGroup := router.Group("/resources")
	pr.setupResourceRoutes(resourcesGroup)

	// Device authorization activation (authenticated, no Casbin menu permission required)
	deviceAuthGroup := router.Group("/auth/device")
	pr.setupDeviceAuthRoutes(deviceAuthGroup)
}

// setupProfileRoutes configures profile management routes
func (pr *ProtectedRoutes) setupProfileRoutes(group *gin.RouterGroup) {
	profileController := pr.factory.CreateProfileController()

	group.GET("/", profileController.GetProfile)
	group.PUT("/", profileController.UpdateProfile)
	group.PUT("/password", profileController.UpdatePassword)
}

// setupResourceRoutes configures resource access routes
func (pr *ProtectedRoutes) setupResourceRoutes(group *gin.RouterGroup) {
	resourceController := pr.factory.CreateResourceController()

	group.GET("", resourceController.GetResources)
}

// setupDeviceAuthRoutes configures authenticated device authorization routes.
func (pr *ProtectedRoutes) setupDeviceAuthRoutes(group *gin.RouterGroup) {
	deviceAuthController := pr.factory.CreateDeviceAuthController()

	group.POST("/activate", deviceAuthController.Activate)
}

// setupSystemRoutes configures system administration routes
func (pr *ProtectedRoutes) setupSystemRoutes(router *gin.RouterGroup, app *bootstrap.Application, engine *gin.Engine) {
	casbinMiddleware := middleware.NewCasbinMiddleware(app.CasManager)
	systemGroup := router.Group("/system")

	// Setup system route groups
	pr.setupUserManagement(systemGroup, casbinMiddleware)
	pr.setupRoleManagement(systemGroup, casbinMiddleware)
	pr.setupMenuManagement(systemGroup, casbinMiddleware)
	pr.setupApiResourceManagement(systemGroup, casbinMiddleware, engine)
	pr.setupLoginLogManagement(systemGroup, casbinMiddleware)
	pr.setupDictionaryManagement(systemGroup, casbinMiddleware)
	pr.setupDepartmentManagement(systemGroup, casbinMiddleware)
}
