package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/constants"
	"shadmin/internal/tokenservice"
	"shadmin/pkg"
)

type loginUsecase struct {
	userRepository           domain.UserRepository
	captchaUsecase           domain.CaptchaUsecase
	loginLogUsecase          domain.LoginLogUseCase
	securityManager          *auth.LoginSecurityManager
	tokenService             *tokenservice.TokenService
	tokenBlacklist           auth.JWTBlacklist
	accessTokenSecret        string
	refreshTokenSecret       string
	accessTokenExpiryMinute  int
	refreshTokenExpiryMinute int
	contextTimeout           time.Duration
}

func NewLoginUsecase(
	userRepository domain.UserRepository,
	captchaUsecase domain.CaptchaUsecase,
	loginLogUsecase domain.LoginLogUseCase,
	securityManager *auth.LoginSecurityManager,
	tokenService *tokenservice.TokenService,
	tokenBlacklist auth.JWTBlacklist,
	accessTokenSecret, refreshTokenSecret string,
	accessTokenExpiryMinute, refreshTokenExpiryMinute int,
	timeout time.Duration,
) domain.LoginUsecase {
	return &loginUsecase{
		userRepository:           userRepository,
		captchaUsecase:           captchaUsecase,
		loginLogUsecase:          loginLogUsecase,
		securityManager:          securityManager,
		tokenService:             tokenService,
		tokenBlacklist:           tokenBlacklist,
		accessTokenSecret:        accessTokenSecret,
		refreshTokenSecret:       refreshTokenSecret,
		accessTokenExpiryMinute:  accessTokenExpiryMinute,
		refreshTokenExpiryMinute: refreshTokenExpiryMinute,
		contextTimeout:           timeout,
	}
}

// Login 执行完整的登录流程：验证码校验 → 锁定检查 → 用户查找 → 状态检查 → 密码校验 → 签发令牌。
// 返回携带错误类型的哨兵错误，由 controller 映射到 HTTP 状态码。
func (lu *loginUsecase) Login(c context.Context, req *domain.LoginRequest, meta domain.LoginMeta) (*domain.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(c, lu.contextTimeout)
	defer cancel()

	if lu.captchaUsecase == nil {
		return nil, errors.New("captcha service not initialized")
	}
	if err := lu.captchaUsecase.VerifySlide(ctx, req.CaptchaID, req.CaptchaX, req.CaptchaY); err != nil {
		return nil, err
	}

	if lu.securityManager == nil {
		return nil, errors.New("security manager not initialized")
	}
	if lu.securityManager.IsLocked(req.Identifier) {
		return nil, lu.accountLockedError(req.Identifier)
	}

	user, err := lu.userRepository.GetByIdentifier(ctx, req.Identifier)
	if err != nil || user == nil {
		lu.securityManager.RecordFailedAttempt(req.Identifier)
		lu.recordLoginLog(meta, constants.StatusFailed, "用户不存在", "")
		return nil, domain.ErrInvalidCredentials
	}

	// 账户状态检查：未启用 / 邀请中 / 已停用 的用户不能登录。
	if user.Status != constants.UserStatusActive {
		lu.recordLoginLog(meta, constants.StatusFailed, "账户未启用或已停用", user.Email)
		return nil, domain.ErrAccountInactive
	}

	// 第三方来源用户没有本地密码：拒绝其走密码登录，避免被撞库。
	if user.Password == "" {
		lu.recordLoginLog(meta, constants.StatusFailed, "第三方账户不支持密码登录", user.Email)
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		lu.securityManager.RecordFailedAttempt(req.Identifier)
		lu.recordLoginLog(meta, constants.StatusFailed, "密码错误", user.Email)

		if lu.securityManager.IsLocked(req.Identifier) {
			return nil, lu.accountLockedError(req.Identifier)
		}

		failedAttempts := lu.securityManager.GetFailedAttempts(req.Identifier)
		remainingAttempts := lu.securityManager.MaxFailures - failedAttempts
		if remainingAttempts > 0 {
			return nil, fmt.Errorf("%w，还可尝试 %d 次", domain.ErrInvalidCredentials, remainingAttempts)
		}
		return nil, domain.ErrInvalidCredentials
	}

	lu.securityManager.RecordSuccessfulLogin(req.Identifier)

	accessToken, err := lu.tokenService.CreateAccessToken(user, lu.accessTokenSecret, lu.accessTokenExpiryMinute)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	refreshToken, err := lu.tokenService.CreateRefreshToken(user, lu.refreshTokenSecret, lu.refreshTokenExpiryMinute)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	lu.recordLoginLog(meta, constants.StatusSuccess, "", user.Email)

	return &domain.LoginResponse{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// Refresh 校验刷新令牌并签发新的访问/刷新令牌。
func (lu *loginUsecase) Refresh(c context.Context, refreshToken string) (*domain.RefreshTokenResponse, error) {
	ctx, cancel := context.WithTimeout(c, lu.contextTimeout)
	defer cancel()

	isValid, err := lu.tokenService.IsAuthorized(refreshToken, lu.refreshTokenSecret)
	if err != nil || !isValid {
		return nil, domain.ErrInvalidRefreshToken
	}

	jti, _ := lu.tokenService.ExtractJTI(refreshToken, lu.refreshTokenSecret)
	revoked, err := lu.tokenBlacklist.Exists(ctx, jti)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}
	if revoked {
		return nil, domain.ErrRefreshTokenRevoked
	}

	userID, err := lu.tokenService.ExtractIDFromToken(refreshToken, lu.refreshTokenSecret)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	user, err := lu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// 刷新令牌前再检查一次状态：admin 可能在 access token 签发后禁用该用户。
	if user.Status != constants.UserStatusActive {
		return nil, domain.ErrAccountInactive
	}

	newAccessToken, err := lu.tokenService.CreateAccessToken(user, lu.accessTokenSecret, lu.accessTokenExpiryMinute)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	newRefreshToken, err := lu.tokenService.CreateRefreshToken(user, lu.refreshTokenSecret, lu.refreshTokenExpiryMinute)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &domain.RefreshTokenResponse{AccessToken: newAccessToken, RefreshToken: newRefreshToken}, nil
}

// Logout 将 access/refresh token 的 jti 加入黑名单，TTL 设为 token 剩余有效期。
func (lu *loginUsecase) Logout(c context.Context, accessToken, refreshToken string) error {
	if accessToken != "" {
		if err := lu.revokeTokenJTI(accessToken, lu.accessTokenSecret); err != nil {
			return err
		}
	}
	if refreshToken != "" {
		if err := lu.revokeTokenJTI(refreshToken, lu.refreshTokenSecret); err != nil {
			return err
		}
	}
	return nil
}

func (lu *loginUsecase) revokeTokenJTI(token, secret string) error {
	jti, exp, _ := lu.tokenService.ExtractJTIAndExpiry(token, secret)
	if time.Now().After(exp) {
		return nil // 已过期的 token 无需加入黑名单
	}
	return lu.tokenBlacklist.Add(context.Background(), jti, exp)
}

func (lu *loginUsecase) accountLockedError(identifier string) error {
	remaining := lu.securityManager.GetRemainingLockTime(identifier)
	return &domain.AccountLockedError{RemainingSeconds: int(remaining.Seconds())}
}

// recordLoginLog 异步记录登录日志，不阻塞登录流程。
func (lu *loginUsecase) recordLoginLog(meta domain.LoginMeta, status, failureReason, email string) {
	if lu.loginLogUsecase == nil {
		return
	}
	logRequest := &domain.CreateLoginLogRequest{
		Email:         email,
		LoginIP:       meta.ClientIP,
		UserAgent:     meta.UserAgent,
		Status:        status,
		Source:        constants.UserSourceLocal,
		FailureReason: failureReason,
	}
	go func() {
		if _, err := lu.loginLogUsecase.CreateLoginLog(context.Background(), logRequest); err != nil {
			pkg.Log.WithError(err).Warn("failed to record login log")
		}
	}()
}

func (lu *loginUsecase) GetUserByIdentifier(c context.Context, identifier string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(c, lu.contextTimeout)
	defer cancel()
	return lu.userRepository.GetByIdentifier(ctx, identifier)
}

func (lu *loginUsecase) GetUserByID(c context.Context, id string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(c, lu.contextTimeout)
	defer cancel()
	return lu.userRepository.GetByID(ctx, id)
}
