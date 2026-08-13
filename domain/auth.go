package domain

import (
	"context"
	"errors"
	"fmt"
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

var (
	ErrInvalidCredentials  = errors.New("用户名或密码错误")
	ErrAccountInactive     = errors.New("账户未启用或已停用，请联系管理员")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenRevoked = errors.New("令牌已登出")
)

// AccountLockedError 账户因连续登录失败被锁定的错误，携带剩余锁定秒数。
type AccountLockedError struct {
	RemainingSeconds int
}

func (e *AccountLockedError) Error() string {
	return fmt.Sprintf("账户已被锁定，请在 %d 秒后重试", e.RemainingSeconds)
}

type LoginUsecase interface {
	Login(c context.Context, req *LoginRequest, meta LoginMeta) (*LoginResponse, error)
	Refresh(c context.Context, refreshToken string) (*RefreshTokenResponse, error)
	Logout(c context.Context, accessToken, refreshToken string) error
	GetUserByIdentifier(c context.Context, identifier string) (*User, error)
	GetUserByID(c context.Context, id string) (*User, error)
}
