package domain

import (
	"context"
	"errors"
	"time"
)

// 社交登录 provider 标识
const (
	ProviderGoogle = "google"
	ProviderGithub = "github"
)

// UserIdentityExchangeRequest 前端用一次性 code 换取 JWT 的请求体
type UserIdentityExchangeRequest struct {
	Code string `json:"code" binding:"required"`
}

// UserIdentity 第三方账号绑定记录。一个用户可绑定多个不同 provider，
// 同一个 provider+provider_subject 全局唯一（对应一个第三方身份）。
type UserIdentity struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Provider        string    `json:"provider"`
	ProviderSubject string    `json:"provider_subject"`
	Email           string    `json:"email,omitempty"`
	Name            string    `json:"name,omitempty"`
	AvatarURL       string    `json:"avatar_url,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserIdentityResult 第三方登录成功后返回给前端的令牌与用户信息
type UserIdentityResult struct {
	AccessToken       string `json:"accessToken"`
	RefreshToken      string `json:"refreshToken"`
	User              *User  `json:"user,omitempty"`
	ProviderAvatarURL string `json:"providerAvatarUrl,omitempty"`
}

// UserIdentityProviderInfo 暴露给前端的已启用 provider 信息
type UserIdentityProviderInfo struct {
	Provider string `json:"provider"`
	Name     string `json:"name"` // 展示名，如 Google / GitHub
}

// UserIdentityProfile 第三方 provider 返回的用户资料。以 domain 自有类型承载，
// 避免 domain 层直接耦合 goth 库；由 controller 从 goth.User 转换而来。
type UserIdentityProfile struct {
	UserID    string // provider 侧的唯一主体 ID（sub / id）
	Email     string
	Name      string
	NickName  string
	AvatarURL string
}

var (
	ErrUserIdentityProviderDisabled = errors.New("user identity provider is not enabled")
	ErrUserIdentityAuthFailed       = errors.New("user identity authentication failed")
)

// UserIdentityRepository 第三方账号绑定存储接口
type UserIdentityRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*UserIdentity, error)
	FindByUserID(ctx context.Context, userID string) ([]*UserIdentity, error)
	Upsert(ctx context.Context, account *UserIdentity) error
}

// UserIdentityUsecase 第三方登录用例接口
type UserIdentityUsecase interface {
	HandleCallback(ctx context.Context, provider string, profile *UserIdentityProfile) (*UserIdentityResult, error)
}
