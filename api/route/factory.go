package route

import (
	"shadmin/api/controller"
	"shadmin/bootstrap"
)

// ControllerFactory 把 bootstrap 装配好的 usecase 组装为 controller。
// 它不构造 repository，也不感知持久化实现；依赖只来自 app.Usecases 与 app.Env。
type ControllerFactory struct {
	app *bootstrap.Application
	uc  *bootstrap.Usecases
}

// NewControllerFactory creates a new controller factory
func NewControllerFactory(app *bootstrap.Application) *ControllerFactory {
	return &ControllerFactory{app: app, uc: app.Usecases}
}

// CreateAuthController creates an authentication controller
func (f *ControllerFactory) CreateAuthController() *controller.AuthController {
	return &controller.AuthController{LoginUsecase: f.uc.Login}
}

// CreateCaptchaController creates a public captcha controller
func (f *ControllerFactory) CreateCaptchaController() *controller.CaptchaController {
	return &controller.CaptchaController{CaptchaUsecase: f.uc.Captcha}
}

// CreateUserIdentityController creates the identity login controller (Google/GitHub OAuth).
func (f *ControllerFactory) CreateUserIdentityController() *controller.UserIdentityController {
	return &controller.UserIdentityController{
		UserIdentityUsecase: f.uc.UserIdentity,
		RedirectURL:         f.app.Env.IdentityRedirectURL,
	}
}

// CreateDeviceAuthController creates a device authorization controller
func (f *ControllerFactory) CreateDeviceAuthController() *controller.DeviceAuthController {
	return controller.NewDeviceAuthController(f.uc.DeviceAuth)
}

// CreateProfileController creates a profile controller
func (f *ControllerFactory) CreateProfileController() *controller.ProfileController {
	return &controller.ProfileController{ProfileUsecase: f.uc.Profile}
}

// CreateResourceController creates a resource controller
func (f *ControllerFactory) CreateResourceController() *controller.ResourceController {
	return &controller.ResourceController{ResourceUsecase: f.uc.Resource}
}

// CreateUserController creates a user controller
func (f *ControllerFactory) CreateUserController() *controller.UserController {
	return &controller.UserController{
		UserUsecase:           f.uc.User,
		UserInvitationUsecase: f.uc.UserInvitation,
	}
}

// CreateUserInvitationController creates a user invitation controller
func (f *ControllerFactory) CreateUserInvitationController() *controller.UserInvitationController {
	return &controller.UserInvitationController{Usecase: f.uc.UserInvitation}
}

// CreateRoleController creates a role controller
func (f *ControllerFactory) CreateRoleController() *controller.RoleController {
	return &controller.RoleController{RoleUseCase: f.uc.Role}
}

// CreateMenuController creates a menu controller
func (f *ControllerFactory) CreateMenuController() *controller.MenuController {
	return &controller.MenuController{MenuUseCase: f.uc.Menu}
}

// CreateApiResourceController creates an API resource controller
func (f *ControllerFactory) CreateApiResourceController() *controller.ApiResourceController {
	return &controller.ApiResourceController{ApiResourceUseCase: f.uc.ApiResource}
}

// CreateLoginLogController creates a login log controller
func (f *ControllerFactory) CreateLoginLogController() *controller.LoginLogController {
	return &controller.LoginLogController{LoginLogUsecase: f.uc.LoginLog}
}

// CreateHealthController creates a health check controller
func (f *ControllerFactory) CreateHealthController() *controller.HealthController {
	return &controller.HealthController{}
}

// CreateDictController creates a dictionary controller
func (f *ControllerFactory) CreateDictController() *controller.DictController {
	return &controller.DictController{DictUseCase: f.uc.Dict}
}

// CreateDepartmentController creates a department controller
func (f *ControllerFactory) CreateDepartmentController() *controller.DepartmentController {
	return &controller.DepartmentController{DepartmentUseCase: f.uc.Department}
}
