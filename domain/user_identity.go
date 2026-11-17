package domain

import (
	"context"
	"errors"
	"time"
)

type UserIdentityExchangeRequest struct {
	Code string `json:"code" binding:"required"`
}

// UserIdentity 的 (provider, provider_subject) 全局唯一。
type UserIdentity struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Provider        string    `json:"provider"`
	ProviderSubject string    `json:"provider_subject"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type UserIdentityResult = LoginResponse

type UserIdentityProfile struct {
	UserID    string // provider 侧的用户标识（subject）
	Email     string
	Name      string
	NickName  string
	AvatarURL string
}

var (
	ErrUserIdentityProviderDisabled = errors.New("user identity provider is not enabled")
	ErrUserIdentityAuthFailed       = errors.New("user identity authentication failed")
	ErrUserIdentityConflict         = errors.New("user identity binding conflict")
	ErrUserIdentityCodeInvalid      = errors.New("identity login code expired or invalid")
)

type UserIdentityRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*UserIdentity, error)
	Upsert(ctx context.Context, account *UserIdentity) error
}

// UserIdentityCodeStore 保存 OAuth 回调后的短期一次性 code。
type UserIdentityCodeStore interface {
	Put(ctx context.Context, result *UserIdentityResult) (string, error)
	// Consume 一次性取出并删除 code；不存在或已过期时返回 (nil, nil)。
	Consume(ctx context.Context, code string) (*UserIdentityResult, error)
}

type UserIdentityUsecase interface {
	// HandleCallback 绑定或创建用户，返回用于换取令牌的一次性 code。
	HandleCallback(ctx context.Context, provider string, profile UserIdentityProfile, meta LoginMeta) (string, error)
	// RecordFailure 记录 provider 认证阶段（尚未拿到资料）的登录失败。
	RecordFailure(ctx context.Context, provider string, meta LoginMeta, reason string)
	// Exchange 用一次性 code 换取令牌；code 无效时返回 ErrUserIdentityCodeInvalid。
	Exchange(ctx context.Context, code string) (*UserIdentityResult, error)
}
