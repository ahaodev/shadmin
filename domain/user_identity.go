package domain

import (
	"context"
	"errors"
	"time"
)

// UserIdentityExchangeRequest 前端用一次性 code 换取 JWT 的请求体
type UserIdentityExchangeRequest struct {
	Code string `json:"code" binding:"required"`
}

// UserIdentity 第三方身份关联记录：同一个 provider+provider_subject 全局唯一，
type UserIdentity struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Provider        string    `json:"provider"`
	ProviderSubject string    `json:"provider_subject"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserIdentityResult 第三方登录成功后返回给前端的令牌与用户信息
type UserIdentityResult = LoginResponse

// UserIdentityProfile 第三方 provider 返回的用户资料（由 controller 从 provider SDK 类型转换而来）。
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

// UserIdentityRepository 第三方身份关联存储接口。
type UserIdentityRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*UserIdentity, error)
	Upsert(ctx context.Context, account *UserIdentity) error
}

// UserIdentityCodeStore 保存 OAuth 回调后的短期一次性 code（由 internal/auth 实现）。
type UserIdentityCodeStore interface {
	Put(ctx context.Context, result *UserIdentityResult) (string, error)
	// Consume 一次性取出并删除 code；不存在或已过期时返回 (nil, nil)。
	Consume(ctx context.Context, code string) (*UserIdentityResult, error)
}

// UserIdentityUsecase 第三方登录用例接口
type UserIdentityUsecase interface {
	// HandleCallback 完成 provider 回调：绑定或创建用户、签发令牌、记录登录日志，返回一次性 code。
	HandleCallback(ctx context.Context, provider string, profile UserIdentityProfile, meta LoginMeta) (string, error)
	// RecordFailure 记录 provider 认证阶段（尚未拿到资料）的登录失败。
	RecordFailure(ctx context.Context, provider string, meta LoginMeta, reason string)
	// Exchange 用一次性 code 换取令牌；code 无效时返回 ErrUserIdentityCodeInvalid。
	Exchange(ctx context.Context, code string) (*UserIdentityResult, error)
}
