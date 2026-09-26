package domain

import (
	"context"
	"errors"
	"time"

	"github.com/markbates/goth"
)

type UserIdentityProvider struct {
	Provider string `json:"provider"` // provider 标识，如 Google / GitHub
	Subject  string `json:"subject"`  // provider 侧的唯一主体 ID（sub / id）
}

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

// UserIdentityProfile 第三方 provider 返回的用户资料。
type UserIdentityProfile = goth.User

var (
	ErrUserIdentityProviderDisabled = errors.New("user identity provider is not enabled")
	ErrUserIdentityAuthFailed       = errors.New("user identity authentication failed")
	ErrUserIdentityConflict         = errors.New("user identity binding conflict")
)

// UserIdentityRepository 第三方身份关联存储接口。
type UserIdentityRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*UserIdentity, error)
	FindByUserID(ctx context.Context, userID string) ([]*UserIdentity, error)
	Upsert(ctx context.Context, account *UserIdentity) error
	WithUserBindingTx(ctx context.Context, fn UserIdentityBindingTxFunc) (*User, error)
}

type UserIdentityBindingTxFunc func(ctx context.Context, userRepo UserRepository, identityRepo UserIdentityRepository, roleRepo RoleRepository) (*User, error)

// UserIdentityUsecase 第三方登录用例接口
type UserIdentityUsecase interface {
	HandleCallback(ctx context.Context, provider string, profile UserIdentityProfile) (*UserIdentityResult, error)
}
