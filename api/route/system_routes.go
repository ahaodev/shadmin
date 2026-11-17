package route

import "github.com/gin-gonic/gin"

func (pr *ProtectedRoutes) setupUserManagement(systemGroup *gin.RouterGroup) {
	userGroup := systemGroup.Group("/user")
	userController := pr.factory.CreateUserController()

	userGroup.GET("", userController.GetUsers)
	userGroup.POST("", userController.CreateUser)

	userGroup.GET("/:id", userController.GetUser)
	userGroup.PUT("/:id", userController.UpdateUser)
	userGroup.DELETE("/:id", userController.DeleteUser)

	userGroup.POST("/invite", userController.InviteUser)
	userGroup.GET("/:id/roles", userController.GetUserRoles)
}

func (pr *ProtectedRoutes) setupRoleManagement(systemGroup *gin.RouterGroup) {
	roleGroup := systemGroup.Group("/role")
	roleController := pr.factory.CreateRoleController()

	roleGroup.GET("", roleController.GetRoles)
	roleGroup.POST("", roleController.CreateRole)
	roleGroup.GET("/:id", roleController.GetRole)
	roleGroup.PUT("/:id", roleController.UpdateRole)
	roleGroup.DELETE("/:id", roleController.DeleteRole)

	roleGroup.GET("/:id/menus", roleController.GetRoleMenus)
}

func (pr *ProtectedRoutes) setupMenuManagement(systemGroup *gin.RouterGroup) {
	menuGroup := systemGroup.Group("/menu")
	menuController := pr.factory.CreateMenuController()

	menuGroup.GET("", menuController.GetMenus)
	menuGroup.POST("", menuController.CreateMenu)
	menuGroup.GET("/:id", menuController.GetMenu)
	menuGroup.PUT("/:id", menuController.UpdateMenu)
	menuGroup.DELETE("/:id", menuController.DeleteMenu)

	menuGroup.GET("/tree", menuController.GetMenuTree)
}

func (pr *ProtectedRoutes) setupApiResourceManagement(systemGroup *gin.RouterGroup) {
	apiGroup := systemGroup.Group("/api-resources")
	apiResourceController := pr.factory.CreateApiResourceController()

	apiGroup.GET("", apiResourceController.GetApiResources)
}

func (pr *ProtectedRoutes) setupLoginLogManagement(systemGroup *gin.RouterGroup) {
	loginLogGroup := systemGroup.Group("/login-logs")
	loginLogController := pr.factory.CreateLoginLogController()

	loginLogGroup.GET("", loginLogController.GetLoginLogs)
	loginLogGroup.DELETE("", loginLogController.ClearLoginLogs)
}

func (pr *ProtectedRoutes) setupDictionaryManagement(systemGroup *gin.RouterGroup) {
	dictGroup := systemGroup.Group("/dict")
	dictController := pr.factory.CreateDictController()

	dictGroup.GET("/types", dictController.GetDictTypes)
	dictGroup.POST("/types", dictController.CreateDictType)
	dictGroup.GET("/types/:id", dictController.GetDictType)
	dictGroup.PUT("/types/:id", dictController.UpdateDictType)
	dictGroup.DELETE("/types/:id", dictController.DeleteDictType)

	dictGroup.GET("/items", dictController.GetDictItems)
	dictGroup.POST("/items", dictController.CreateDictItem)
	dictGroup.GET("/items/:id", dictController.GetDictItem)
	dictGroup.PUT("/items/:id", dictController.UpdateDictItem)
	dictGroup.DELETE("/items/:id", dictController.DeleteDictItem)

	dictGroup.GET("/types/code/:code/items", dictController.GetDictItemsByTypeCode)
}

func (pr *ProtectedRoutes) setupDepartmentManagement(systemGroup *gin.RouterGroup) {
	deptGroup := systemGroup.Group("/department")
	deptController := pr.factory.CreateDepartmentController()

	deptGroup.GET("", deptController.GetDepartmentList)
	deptGroup.GET("/tree", deptController.GetDepartmentTree)
	deptGroup.POST("", deptController.CreateDepartment)
	deptGroup.GET("/:id", deptController.GetDepartment)
	deptGroup.PUT("/:id", deptController.UpdateDepartment)
	deptGroup.DELETE("/:id", deptController.DeleteDepartment)
}
