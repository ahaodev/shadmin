package bootstrap

import (
	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/mailer"
	"shadmin/internal/tokenservice"
	"shadmin/repository"
	"shadmin/usecase"
	"time"
)

// Usecases 是应用层全部用例实例，由 newUsecases 统一装配。
// api 层只取用这些实例，不构造 repository，也不感知持久化实现。
type Usecases struct {
	Login          domain.LoginUsecase
	Captcha        domain.CaptchaUsecase
	UserIdentity   domain.UserIdentityUsecase
	DeviceAuth     domain.DeviceAuthUsecase
	Profile        domain.ProfileUsecase
	Resource       domain.ResourceUseCase
	User           domain.UserUseCase
	UserInvitation domain.UserInvitationUseCase
	Role           domain.RoleUseCase
	Menu           domain.MenuUseCase
	ApiResource    domain.ApiResourceUseCase
	LoginLog       domain.LoginLogUseCase
	Dict           domain.DictUseCase
	Department     domain.DepartmentUseCase
}

// newUsecases 依赖注入的唯一位置：repository → usecase。调用前 app 的 DB、Cacher、
// CaptchaManager、TokenBlacklist 与 Env 必须已经初始化。
func newUsecases(app *Application) *Usecases {
	env := app.Env
	db := app.DB
	timeout := time.Duration(env.ContextTimeout) * time.Second

	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	menuRepo := repository.NewMenuRepository(db)
	loginLog := usecase.NewLoginLogUsecase(repository.NewLoginLogRepository(db), timeout)
	tokens := tokenservice.NewTokenService()
	captcha := usecase.NewCaptchaUsecase(app.CaptchaManager, timeout)

	var emailSender domain.InvitationMailer
	if env.ResendKey != "" {
		emailSender = mailer.NewResendInvitationMailer(env.ResendKey, env.ResendFromEmail)
	}

	return &Usecases{
		Login: usecase.NewLoginUsecase(
			userRepo,
			captcha,
			loginLog,
			auth.NewLoginSecurityManager(app.Cacher),
			tokens,
			app.TokenBlacklist,
			env.AccessTokenSecret,
			env.RefreshTokenSecret,
			env.AccessTokenExpiryMinute,
			env.RefreshTokenExpiryMinute,
			timeout,
		),
		Captcha: captcha,
		UserIdentity: usecase.NewUserIdentityUsecase(
			repository.NewUnitOfWork(db),
			userRepo,
			roleRepo,
			repository.NewUserIdentityRepository(db),
			tokens,
			auth.NewUserIdentityCodeStore(app.Cacher, 3*time.Minute),
			loginLog,
			env.AccessTokenSecret,
			env.RefreshTokenSecret,
			env.AccessTokenExpiryMinute,
			env.RefreshTokenExpiryMinute,
			timeout,
		),
		DeviceAuth: usecase.NewDeviceAuthUsecase(
			repository.NewDeviceAuthRepository(db),
			userRepo,
			tokens,
			env.AccessTokenSecret,
			env.RefreshTokenSecret,
			env.AccessTokenExpiryMinute,
			env.RefreshTokenExpiryMinute,
			timeout,
		),
		Profile:  usecase.NewProfileUsecase(repository.NewProfileRepository(db), timeout),
		Resource: usecase.NewResourceUsecase(menuRepo, userRepo, roleRepo, timeout),
		User:     usecase.NewUserUsecase(userRepo, timeout),
		UserInvitation: usecase.NewUserInvitationUsecase(
			repository.NewUserInvitationRepository(db),
			roleRepo,
			emailSender,
			env.InvitationAcceptURL,
			time.Duration(env.InvitationExpiryHours)*time.Hour,
			timeout,
		),
		Role:        usecase.NewRoleUsecase(roleRepo, timeout),
		Menu:        usecase.NewMenuUsecase(menuRepo, timeout),
		ApiResource: usecase.NewApiResourceUsecase(repository.NewApiResourceRepository(db), timeout),
		LoginLog:    loginLog,
		Dict:        usecase.NewDictUsecase(repository.NewDictRepository(db), timeout),
		Department:  usecase.NewDepartmentUsecase(repository.NewDepartmentRepository(db), timeout),
	}
}
