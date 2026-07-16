package route

import (
	"log"
	"shadmin/api/controller"
	"shadmin/bootstrap"
	"shadmin/internal/auth"
	captchapkg "shadmin/internal/captcha"
	"shadmin/internal/casbin"
	"shadmin/internal/tokenservice"
	"shadmin/repository"
	"shadmin/usecase"
	"time"

	"shadmin/ent"

	"github.com/gin-gonic/gin"
)

// ControllerFactory creates and manages controller instances
type ControllerFactory struct {
	app                  *bootstrap.Application
	timeout              time.Duration
	db                   *ent.Client
	captchaManager       *captchapkg.SlideManager
	deviceAuthController *controller.DeviceAuthController
}

// NewControllerFactory creates a new controller factory
func NewControllerFactory(app *bootstrap.Application, timeout time.Duration, db *ent.Client) *ControllerFactory {
	if app.CaptchaManager == nil {
		log.Fatalf("bootstrap: captcha manager not initialized")
	}
	return &ControllerFactory{
		app:            app,
		timeout:        timeout,
		db:             db,
		captchaManager: app.CaptchaManager,
	}
}

// CreateAuthController creates an authentication controller
func (f *ControllerFactory) CreateAuthController(casManager casbin.Manager) *controller.AuthController {
	ur := repository.NewUserRepository(f.db, casManager)
	ts := tokenservice.NewTokenService()

	// 创建LoginLog相关的repository和usecase
	loginLogRepository := repository.NewLoginLogRepository(f.db)
	loginLogUseCase := usecase.NewLoginLogUsecase(loginLogRepository, f.timeout)

	return &controller.AuthController{
		LoginUsecase:    usecase.NewLoginUsecase(ur, f.timeout),
		LoginLogUsecase: loginLogUseCase,
		Env:             f.app.Env,
		SecurityManager: auth.NewLoginSecurityManager(),
		TokenService:    ts,
		CaptchaUsecase:  usecase.NewCaptchaUsecase(f.captchaManager, f.timeout),
		TokenBlacklist:  f.app.TokenBlacklist,
	}
}

// CreateCaptchaController creates a public captcha controller
func (f *ControllerFactory) CreateCaptchaController() *controller.CaptchaController {
	return &controller.CaptchaController{
		CaptchaUsecase: usecase.NewCaptchaUsecase(f.captchaManager, f.timeout),
	}
}

// CreateSocialAuthController creates the social login controller (Google/GitHub OAuth).
// 复用既有 TokenService + env 令牌密钥签发 JWT，绑定记录走 SocialAccountRepository。
func (f *ControllerFactory) CreateSocialAuthController() *controller.SocialAuthController {
	userRepo := repository.NewUserRepository(f.db, f.app.CasManager)
	socialAccountRepo := repository.NewSocialAccountRepository(f.db)
	tokenService := tokenservice.NewTokenService()
	return &controller.SocialAuthController{
		SocialLoginUsecase: usecase.NewSocialLoginUsecase(
			userRepo,
			socialAccountRepo,
			tokenService,
			f.app.Env.AccessTokenSecret,
			f.app.Env.RefreshTokenSecret,
			f.app.Env.AccessTokenExpiryMinute,
			f.app.Env.RefreshTokenExpiryMinute,
			f.timeout,
		),
		RedirectURL: f.app.Env.SocialRedirectURL,
	}
}

// CreateDeviceAuthController creates a device authorization controller
func (f *ControllerFactory) CreateDeviceAuthController() *controller.DeviceAuthController {
	if f.deviceAuthController != nil {
		return f.deviceAuthController
	}
	deviceAuthRepository := repository.NewDeviceAuthRepository(f.db)
	userRepository := repository.NewUserRepository(f.db, f.app.CasManager)
	tokenService := tokenservice.NewTokenService()

	f.deviceAuthController = controller.NewDeviceAuthController(
		usecase.NewDeviceAuthUsecase(
			deviceAuthRepository,
			userRepository,
			tokenService,
			f.app.Env.AccessTokenSecret,
			f.app.Env.RefreshTokenSecret,
			f.app.Env.AccessTokenExpiryMinute,
			f.app.Env.RefreshTokenExpiryMinute,
			f.timeout,
		),
	)
	return f.deviceAuthController
}

// CreateProfileController creates a profile controller
func (f *ControllerFactory) CreateProfileController() *controller.ProfileController {
	pr := repository.NewProfileRepository(f.db)

	return &controller.ProfileController{
		ProfileUsecase: usecase.NewProfileUsecase(pr, f.timeout),
		Env:            f.app.Env,
	}
}

// CreateResourceController creates a resource controller
func (f *ControllerFactory) CreateResourceController() *controller.ResourceController {
	menuRepository := repository.NewMenuRepository(f.db)
	roleRepository := repository.NewRoleRepository(f.db)
	userRepository := repository.NewUserRepository(f.db, f.app.CasManager)

	return &controller.ResourceController{
		MenuRepository: menuRepository,
		RoleRepository: roleRepository,
		UserRepository: userRepository,
		Env:            f.app.Env,
	}
}

// CreateUserController creates a user controller
func (f *ControllerFactory) CreateUserController() *controller.UserController {
	ur := repository.NewUserRepository(f.db, f.app.CasManager)
	rr := repository.NewRoleRepository(f.db)

	return &controller.UserController{
		UserUsecase: usecase.NewUserUsecase(f.db, ur, rr, f.timeout),
		Env:         f.app.Env,
	}
}

// CreateRoleController creates a role controller
func (f *ControllerFactory) CreateRoleController() *controller.RoleController {
	roleRepository := repository.NewRoleRepository(f.db)
	userRepository := repository.NewUserRepository(f.db, f.app.CasManager)
	menuRepository := repository.NewMenuRepository(f.db)
	roleUseCase := usecase.NewRoleUsecase(f.db, roleRepository, f.timeout)

	return &controller.RoleController{
		CasManager:     f.app.CasManager,
		Env:            f.app.Env,
		RoleUseCase:    roleUseCase,
		UserRepository: userRepository,
		RoleRepository: roleRepository,
		MenuRepository: menuRepository,
	}
}

// CreateMenuController creates a menu controller
func (f *ControllerFactory) CreateMenuController() *controller.MenuController {
	menuRepository := repository.NewMenuRepository(f.db)
	menuUsecase := usecase.NewMenuUsecase(f.db, menuRepository, f.timeout)

	return &controller.MenuController{
		MenuUseCase: menuUsecase,
		Env:         f.app.Env,
	}
}

// CreateApiResourceController creates an API resource controller
func (f *ControllerFactory) CreateApiResourceController(engine *gin.Engine) *controller.ApiResourceController {
	apiResourceRepository := repository.NewApiResourceRepository(f.db)
	apiResourceUseCase := usecase.NewApiResourceUsecase(apiResourceRepository, engine, f.timeout)

	return &controller.ApiResourceController{
		ApiResourceUseCase: apiResourceUseCase,
	}
}

// CreateLoginLogController creates a login log controller
func (f *ControllerFactory) CreateLoginLogController() *controller.LoginLogController {
	loginLogRepository := repository.NewLoginLogRepository(f.db)
	loginLogUseCase := usecase.NewLoginLogUsecase(loginLogRepository, f.timeout)

	return &controller.LoginLogController{
		LoginLogUsecase: loginLogUseCase,
		Env:             f.app.Env,
	}
}

// CreateHealthController creates a health check controller
func (f *ControllerFactory) CreateHealthController() *controller.HealthController {
	return &controller.HealthController{}
}

// CreateDictController creates a dictionary controller
func (f *ControllerFactory) CreateDictController() *controller.DictController {
	dictRepository := repository.NewDictRepository(f.db)
	dictUseCase := usecase.NewDictUsecase(f.db, dictRepository, f.timeout)

	return &controller.DictController{
		DictUseCase: dictUseCase,
	}
}

// CreateDepartmentController creates a department controller
func (f *ControllerFactory) CreateDepartmentController() *controller.DepartmentController {
	departmentRepository := repository.NewDepartmentRepository(f.db)
	departmentUseCase := usecase.NewDepartmentUsecase(departmentRepository, f.timeout)

	return &controller.DepartmentController{
		DepartmentUseCase: departmentUseCase,
		Env:               f.app.Env,
	}
}
