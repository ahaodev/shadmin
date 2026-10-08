package domain

import (
	"context"
	"errors"
	"time"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Identifier string `form:"identifier" json:"identifier" binding:"required"`
	Password   string `form:"password" json:"password" binding:"required"`
	CaptchaID  string `form:"captcha_id" json:"captcha_id" binding:"required"`
	CaptchaX   int    `form:"captcha_x" json:"captcha_x"`
	CaptchaY   int    `form:"captcha_y" json:"captcha_y"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" form:"refreshToken" binding:"required"`
}

// RefreshTokenResponse 刷新令牌响应
type RefreshTokenResponse = LoginResponse

// ProfileUpdate 个人资料更新请求
type ProfileUpdate struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Bio    string `json:"bio"`
}

// PasswordUpdate 密码更新请求
type PasswordUpdate struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"` // 可选的刷新令牌，用于更完整的登出处理
}

// LoginMeta 登录请求的元信息（客户端 IP 与 User-Agent），由 controller 提取后传入 usecase。
type LoginMeta struct {
	ClientIP  string
	UserAgent string
}

// TokenClaims 是应用层视角的令牌声明，不暴露 JWT 库类型。
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

// TokenIssuer 签发与解析令牌（由 internal/tokenservice 实现）。
type TokenIssuer interface {
	CreateAccessToken(user *User, secret string, expiry int) (string, error)
	CreateAccessTokenWithIdentity(user *User, secret string, expiry int, provider, providerSubject, source string) (string, error)
	CreateRefreshToken(user *User, secret string, expiry int) (string, error)
	ParseAccessClaims(token, secret string) (*TokenClaims, error)
	ParseRefreshClaims(token, secret string) (*TokenClaims, error)
	ExtractJTIAndExpiry(token, secret string) (string, time.Time, bool)
}

// TokenBlacklist 记录已登出令牌的 jti，直到其原始过期时间（由 internal/auth 实现）。
type TokenBlacklist interface {
	// Add 将 jti 加入黑名单直到 expiry；expiry 已过则直接忽略。
	Add(ctx context.Context, jti string, expiry time.Time) error
	// Exists 检查 jti 是否在黑名单中且仍有效。
	Exists(ctx context.Context, jti string) (bool, error)
}

// LoginSecurity 按账号统计连续登录失败次数（由 internal/auth 实现）。
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

// AccountLockedError 账户因连续登录失败被锁定。
type AccountLockedError struct{}

func (e *AccountLockedError) Error() string {
	return "账户已被锁定，请稍后重试"
}

type LoginUsecase interface {
	Login(c context.Context, req *LoginRequest, meta LoginMeta) (*LoginResponse, error)
	Refresh(c context.Context, refreshToken string) (*RefreshTokenResponse, error)
	Logout(c context.Context, accessToken, refreshToken string) error
	GetUserByIdentifier(c context.Context, identifier string) (*User, error)
	GetUserByID(c context.Context, id string) (*User, error)
}
