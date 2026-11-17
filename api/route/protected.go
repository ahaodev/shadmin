package route

import (
	"time"

	"shadmin/api/middleware"
	"shadmin/bootstrap"

	"github.com/gin-gonic/gin"
)

type ProtectedRoutes struct {
	factory *ControllerFactory
}

func NewProtectedRoutes(factory *ControllerFactory) *ProtectedRoutes {
	return &ProtectedRoutes{
		factory: factory,
	}
}

func (pr *ProtectedRoutes) Setup(router *gin.RouterGroup, app *bootstrap.Application) {
	protectedRouter := router.Group("")
	protectedRouter.Use(middleware.JwtAuthMiddleware(app.Tokens, app.TokenBlacklist))
	protectedRouter.Use(middleware.UserStateMiddleware(app.UserStatusCache))

	pr.setupUserRoutes(protectedRouter, app)
	pr.setupSystemRoutes(protectedRouter, app)
}

func (pr *ProtectedRoutes) setupUserRoutes(router *gin.RouterGroup, app *bootstrap.Application) {
	profileGroup := router.Group("/profile")
	pr.setupProfileRoutes(profileGroup)

	resourcesGroup := router.Group("/resources")
	pr.setupResourceRoutes(resourcesGroup)

	// 设备激活仅需认证，无需菜单权限。
	deviceAuthGroup := router.Group("/auth/device")
	pr.setupDeviceAuthRoutes(deviceAuthGroup)
}

func (pr *ProtectedRoutes) setupProfileRoutes(group *gin.RouterGroup) {
	profileController := pr.factory.CreateProfileController()

	group.GET("", profileController.GetProfile)
	group.PUT("", profileController.UpdateProfile)
	group.PUT("/password", profileController.UpdatePassword)
}

func (pr *ProtectedRoutes) setupResourceRoutes(group *gin.RouterGroup) {
	resourceController := pr.factory.CreateResourceController()

	group.GET("", resourceController.GetResources)
}

func (pr *ProtectedRoutes) setupDeviceAuthRoutes(group *gin.RouterGroup) {
	deviceAuthController := pr.factory.CreateDeviceAuthController()

	group.POST("/activate", middleware.RateLimitByIP(20, time.Minute), deviceAuthController.Activate)
}

func (pr *ProtectedRoutes) setupSystemRoutes(router *gin.RouterGroup, app *bootstrap.Application) {
	casbinMiddleware := middleware.NewCasbinMiddleware(app.CasbinInitializer.GetSyncService())
	systemGroup := router.Group("/system")
	systemGroup.Use(casbinMiddleware.CheckAPIPermission())

	pr.setupUserManagement(systemGroup)
	pr.setupRoleManagement(systemGroup)
	pr.setupMenuManagement(systemGroup)
	pr.setupApiResourceManagement(systemGroup)
	pr.setupLoginLogManagement(systemGroup)
	pr.setupDictionaryManagement(systemGroup)
	pr.setupDepartmentManagement(systemGroup)
}
