package domain

import (
	"context"
	"errors"
	"time"
)

// 社交登录 provider 标识
const (
	SocialGoogle = "google"
	SocialGitHub = "github"
)

// SocialAccount 第三方账号绑定记录。一个用户可绑定多个不同 provider，
// 同一个 provider+provider_subject 全局唯一（对应一个第三方身份）。
type SocialAccount struct {
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

// SocialLoginResult 第三方登录成功后返回给前端的令牌与用户信息
type SocialLoginResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         *User  `json:"user,omitempty"`
}

// SocialProviderInfo 暴露给前端的已启用 provider 信息
type SocialProviderInfo struct {
	Provider string `json:"provider"`
	Name     string `json:"name"` // 展示名，如 Google / GitHub
}

// SocialProfile 第三方 provider 返回的用户资料。以 domain 自有类型承载，
// 避免 domain 层直接耦合 goth 库；由 controller 从 goth.User 转换而来。
type SocialProfile struct {
	UserID    string // provider 侧的唯一主体 ID（sub / id）
	Email     string
	Name      string
	NickName  string
	AvatarURL string
}

var (
	ErrSocialProviderDisabled = errors.New("social provider is not enabled")
	ErrSocialAuthFailed       = errors.New("social authentication failed")
)

// SocialAccountRepository 第三方账号绑定存储接口
type SocialAccountRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*SocialAccount, error)
	FindByUserID(ctx context.Context, userID string) ([]*SocialAccount, error)
	Upsert(ctx context.Context, account *SocialAccount) error
}

// SocialLoginUsecase 第三方登录用例接口
type SocialLoginUsecase interface {
	HandleCallback(ctx context.Context, provider string, profile *SocialProfile) (*SocialLoginResult, error)
}
