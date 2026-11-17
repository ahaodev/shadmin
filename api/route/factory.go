package route

import (
	"shadmin/api/controller"
	"shadmin/bootstrap"
)

type ControllerFactory struct {
	app *bootstrap.Application
	uc  *bootstrap.Usecases
}

func NewControllerFactory(app *bootstrap.Application) *ControllerFactory {
	return &ControllerFactory{app: app, uc: app.Usecases}
}

func (f *ControllerFactory) CreateAuthController() *controller.AuthController {
	return &controller.AuthController{LoginUsecase: f.uc.Login}
}

func (f *ControllerFactory) CreateCaptchaController() *controller.CaptchaController {
	return &controller.CaptchaController{CaptchaUsecase: f.uc.Captcha}
}

func (f *ControllerFactory) CreateUserIdentityController() *controller.UserIdentityController {
	return &controller.UserIdentityController{
		UserIdentityUsecase: f.uc.UserIdentity,
		RedirectURL:         f.app.Env.IdentityRedirectURL,
	}
}

func (f *ControllerFactory) CreateDeviceAuthController() *controller.DeviceAuthController {
	return controller.NewDeviceAuthController(f.uc.DeviceAuth)
}

func (f *ControllerFactory) CreateProfileController() *controller.ProfileController {
	return &controller.ProfileController{ProfileUsecase: f.uc.Profile}
}

func (f *ControllerFactory) CreateResourceController() *controller.ResourceController {
	return &controller.ResourceController{ResourceUsecase: f.uc.Resource}
}

func (f *ControllerFactory) CreateUserController() *controller.UserController {
	return &controller.UserController{
		UserUsecase:           f.uc.User,
		UserInvitationUsecase: f.uc.UserInvitation,
	}
}

func (f *ControllerFactory) CreateUserInvitationController() *controller.UserInvitationController {
	return &controller.UserInvitationController{Usecase: f.uc.UserInvitation}
}

func (f *ControllerFactory) CreateRoleController() *controller.RoleController {
	return &controller.RoleController{RoleUseCase: f.uc.Role}
}

func (f *ControllerFactory) CreateMenuController() *controller.MenuController {
	return &controller.MenuController{MenuUseCase: f.uc.Menu}
}

func (f *ControllerFactory) CreateApiResourceController() *controller.ApiResourceController {
	return &controller.ApiResourceController{ApiResourceUseCase: f.uc.ApiResource}
}

func (f *ControllerFactory) CreateLoginLogController() *controller.LoginLogController {
	return &controller.LoginLogController{LoginLogUsecase: f.uc.LoginLog}
}

func (f *ControllerFactory) CreateHealthController() *controller.HealthController {
	return &controller.HealthController{}
}

func (f *ControllerFactory) CreateDictController() *controller.DictController {
	return &controller.DictController{DictUseCase: f.uc.Dict}
}

func (f *ControllerFactory) CreateDepartmentController() *controller.DepartmentController {
	return &controller.DepartmentController{DepartmentUseCase: f.uc.Department}
}
