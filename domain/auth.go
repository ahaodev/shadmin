package domain

import (
	"context"
	"errors"
	"time"
)

type LoginRequest struct {
	Identifier string `form:"identifier" json:"identifier" binding:"required"`
	Password   string `form:"password" json:"password" binding:"required"`
	CaptchaID  string `form:"captcha_id" json:"captcha_id" binding:"required"`
	CaptchaX   int    `form:"captcha_x" json:"captcha_x"`
	CaptchaY   int    `form:"captcha_y" json:"captcha_y"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" form:"refreshToken" binding:"required"`
}

type RefreshTokenResponse = LoginResponse

type ProfileUpdate struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Bio    string `json:"bio"`
}

type PasswordUpdate struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type LoginMeta struct {
	ClientIP  string
	UserAgent string
}

// ID 为用户 ID，JTI 为登出黑名单的键；刷新令牌只填充 ID/JTI/ExpiresAt。
type TokenClaims struct {
	ID        string
	Name      string
	Email     string
	IsAdmin   bool
	Roles     []string
	Subject   string
	JTI       string
	ExpiresAt time.Time
}

type TokenIssuer interface {
	IssuePair(user *User) (*LoginResponse, error)
	// IssuePairWithIdentity 设置 sub 为 provider:providerSubject。
	IssuePairWithIdentity(user *User, provider, providerSubject string) (*LoginResponse, error)
	ParseAccessClaims(token string) (*TokenClaims, error)
	ParseRefreshClaims(token string) (*TokenClaims, error)
	// ok=false 表示令牌不可用或无 exp。
	ExtractAccessJTI(token string) (jti string, expiresAt time.Time, ok bool)
	ExtractRefreshJTI(token string) (jti string, expiresAt time.Time, ok bool)
}

// TokenBlacklist 按 jti 吊销令牌，保留至令牌过期。
type TokenBlacklist interface {
	// Consume atomically revokes a live JTI; only the first caller returns true.
	Consume(ctx context.Context, jti string, expiry time.Time) (bool, error)
	// Add 忽略已过期的令牌。
	Add(ctx context.Context, jti string, expiry time.Time) error
	Exists(ctx context.Context, jti string) (bool, error)
}

type LoginSecurity interface {
	IsLocked(ctx context.Context, identifier string) bool
	RecordFailedAttempt(ctx context.Context, identifier string) int
	RecordSuccessfulLogin(ctx context.Context, identifier string)
	MaxFailures() int
}

var (
	ErrInvalidCredentials  = errors.New("用户名或密码错误")
	ErrAccountInactive     = errors.New("账户未启用或已停用，请联系管理员")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenRevoked = errors.New("令牌已登出")
)

type AccountLockedError struct{}

func (e *AccountLockedError) Error() string {
	return "账户已被锁定，请稍后重试"
}

type LoginUsecase interface {
	Login(ctx context.Context, req *LoginRequest, meta LoginMeta) (*LoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*RefreshTokenResponse, error)
	Logout(ctx context.Context, accessToken, refreshToken string) error
}
