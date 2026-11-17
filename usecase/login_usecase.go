package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"shadmin/domain"
)

type loginUsecase struct {
	userRepository  domain.UserRepository
	captchaUsecase  domain.CaptchaUsecase
	loginLogUsecase domain.LoginLogUseCase
	securityManager domain.LoginSecurity
	tokenService    domain.TokenIssuer
	tokenBlacklist  domain.TokenBlacklist
	contextTimeout  time.Duration
}

func NewLoginUsecase(
	userRepository domain.UserRepository,
	captchaUsecase domain.CaptchaUsecase,
	loginLogUsecase domain.LoginLogUseCase,
	securityManager domain.LoginSecurity,
	tokenService domain.TokenIssuer,
	tokenBlacklist domain.TokenBlacklist,
	timeout time.Duration,
) domain.LoginUsecase {
	return &loginUsecase{
		userRepository:  userRepository,
		captchaUsecase:  captchaUsecase,
		loginLogUsecase: loginLogUsecase,
		securityManager: securityManager,
		tokenService:    tokenService,
		tokenBlacklist:  tokenBlacklist,
		contextTimeout:  timeout,
	}
}

func (lu *loginUsecase) Login(ctx context.Context, req *domain.LoginRequest, meta domain.LoginMeta) (*domain.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, lu.contextTimeout)
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
	if lu.securityManager.IsLocked(ctx, req.Identifier) {
		return nil, lu.accountLockedError()
	}

	user, err := lu.userRepository.GetByIdentifier(ctx, req.Identifier)
	if errors.Is(err, domain.ErrUserNotFound) {
		lu.securityManager.RecordFailedAttempt(ctx, req.Identifier)
		lu.recordLoginLog(ctx, meta, domain.StatusFailed, "用户不存在", "")
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("get user by identifier: %w", err)
	}

	if user.Status != domain.UserStatusActive {
		lu.recordLoginLog(ctx, meta, domain.StatusFailed, "账户未启用或已停用", user.Email)
		return nil, domain.ErrAccountInactive
	}

	if user.Password == "" {
		lu.recordLoginLog(ctx, meta, domain.StatusFailed, "第三方账户不支持密码登录", user.Email)
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		attempts := lu.securityManager.RecordFailedAttempt(ctx, req.Identifier)
		lu.recordLoginLog(ctx, meta, domain.StatusFailed, "密码错误", user.Email)

		if attempts >= lu.securityManager.MaxFailures() {
			return nil, lu.accountLockedError()
		}
		return nil, fmt.Errorf("%w，还可尝试 %d 次", domain.ErrInvalidCredentials, lu.securityManager.MaxFailures()-attempts)
	}

	lu.securityManager.RecordSuccessfulLogin(ctx, req.Identifier)

	tokens, err := lu.tokenService.IssuePair(user)
	if err != nil {
		return nil, err
	}

	lu.recordLoginLog(ctx, meta, domain.StatusSuccess, "", user.Email)

	return tokens, nil
}

func (lu *loginUsecase) Refresh(ctx context.Context, refreshToken string) (*domain.RefreshTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, lu.contextTimeout)
	defer cancel()

	claims, err := lu.tokenService.ParseRefreshClaims(refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	if lu.tokenBlacklist == nil || claims.JTI == "" || !claims.ExpiresAt.After(time.Now()) {
		return nil, domain.ErrInvalidRefreshToken
	}

	user, err := lu.userRepository.GetByID(ctx, claims.ID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil, domain.ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, fmt.Errorf("get user for refresh: %w", err)
	}

	// 签发后用户可能已被禁用，刷新时重新检查状态。
	if user.Status != domain.UserStatusActive {
		return nil, domain.ErrAccountInactive
	}

	tokens, err := lu.tokenService.IssuePair(user)
	if err != nil {
		return nil, err
	}

	// Generate before consuming, but return tokens only to the atomic winner.
	consumed, err := lu.tokenBlacklist.Consume(ctx, claims.JTI, claims.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("consume refresh token: %w", err)
	}
	if !consumed {
		return nil, domain.ErrRefreshTokenRevoked
	}

	return tokens, nil
}

// Logout 吊销 access/refresh token，保留至各自过期。
func (lu *loginUsecase) Logout(ctx context.Context, accessToken, refreshToken string) error {
	ctx, cancel := context.WithTimeout(ctx, lu.contextTimeout)
	defer cancel()

	if accessToken != "" {
		jti, exp, ok := lu.tokenService.ExtractAccessJTI(accessToken)
		if err := lu.revoke(ctx, jti, exp, ok); err != nil {
			return err
		}
	}
	if refreshToken != "" {
		jti, exp, ok := lu.tokenService.ExtractRefreshJTI(refreshToken)
		if err := lu.revoke(ctx, jti, exp, ok); err != nil {
			return err
		}
	}
	return nil
}

func (lu *loginUsecase) revoke(ctx context.Context, jti string, exp time.Time, ok bool) error {
	if !ok || jti == "" || time.Now().After(exp) {
		return nil
	}
	return lu.tokenBlacklist.Add(ctx, jti, exp)
}

func (lu *loginUsecase) accountLockedError() error {
	return &domain.AccountLockedError{}
}

func (lu *loginUsecase) recordLoginLog(parent context.Context, meta domain.LoginMeta, status, failureReason, email string) {
	writeLoginLog(parent, lu.loginLogUsecase, meta, domain.UserSourceLocal, status, failureReason, email)
}
