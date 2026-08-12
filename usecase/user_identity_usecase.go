package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"shadmin/internal/constants"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/internal/tokenservice"
)

type userIdentityUsecase struct {
	identityRepository domain.UserIdentityRepository
	tokenService       *tokenservice.TokenService
	accessTokenSecret  string
	refreshTokenSecret string
	accessTokenExpiry  int
	refreshTokenExpiry int
	contextTimeout     time.Duration
}

// NewUserIdentityUsecase 构造第三方登录用例。
// token 相关参数与 device_auth_usecase 保持一致：复用同一套 TokenService + env secrets，
// 不引入独立的令牌签发流程。
func NewUserIdentityUsecase(
	identityRepository domain.UserIdentityRepository,
	tokenService *tokenservice.TokenService,
	accessTokenSecret, refreshTokenSecret string,
	accessTokenExpiry, refreshTokenExpiry int,
	timeout time.Duration,
) domain.UserIdentityUsecase {
	return &userIdentityUsecase{
		identityRepository: identityRepository,
		tokenService:       tokenService,
		accessTokenSecret:  accessTokenSecret,
		refreshTokenSecret: refreshTokenSecret,
		accessTokenExpiry:  accessTokenExpiry,
		refreshTokenExpiry: refreshTokenExpiry,
		contextTimeout:     timeout,
	}
}

// HandleCallback 处理 provider 回调：解析第三方 profile，查找/创建用户，
// 复用既有 JWT 体系签发令牌对。
// 登录只认 (provider, provider_subject) 关联：已关联 → 登录该用户并刷新资料；
// 未关联 → 创建独立用户（不按 email 合并，与本地用户完全隔离）。
func (u *userIdentityUsecase) HandleCallback(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.UserIdentityResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required: %w", domain.ErrUserIdentityAuthFailed)
	}
	if strings.TrimSpace(profile.UserID) == "" {
		return nil, fmt.Errorf("provider %s returned empty subject: %w", provider, domain.ErrUserIdentityAuthFailed)
	}

	user, err := u.resolveOrCreateUser(ctx, provider, profile)
	if err != nil {
		return nil, err
	}

	// 复用既有 TokenService 签发 JWT 令牌对，并把第三方身份信息写入 access token。
	accessToken, err := u.tokenService.CreateAccessTokenWithIdentity(user, u.accessTokenSecret, u.accessTokenExpiry, provider, profile.UserID, "oidc")
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	refreshToken, err := u.tokenService.CreateRefreshToken(user, u.refreshTokenSecret, u.refreshTokenExpiry)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &domain.UserIdentityResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// resolveOrCreateUser 解析 (provider, provider_subject) 对应的用户：
// 已关联 → 返回关联用户并刷新资料；未关联 → 创建独立用户并建立关联记录。
// 并发首次登录可能同时建号，唯一约束冲突时重试一次。
func (u *userIdentityUsecase) resolveOrCreateUser(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		user, err := u.resolveOrCreateUserOnce(ctx, provider, profile)
		if err == nil {
			return user, nil
		}
		lastErr = err
		if !isUniqueViolation(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (u *userIdentityUsecase) resolveOrCreateUserOnce(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	return u.identityRepository.WithUserBindingTx(ctx, func(txCtx context.Context, userRepo domain.UserRepository, identityRepo domain.UserIdentityRepository) (*domain.User, error) {
		return u.resolveOrCreateUserForIdentity(txCtx, userRepo, identityRepo, provider, profile)
	})
}

func (u *userIdentityUsecase) resolveOrCreateUserForIdentity(
	ctx context.Context,
	userRepo domain.UserRepository,
	identityRepo domain.UserIdentityRepository,
	provider string,
	profile domain.UserIdentityProfile,
) (*domain.User, error) {
	// 1. 先查该 (provider, provider_subject) 是否已关联
	account, err := identityRepo.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find identity account: %w", err)
	}

	if account != nil {
		// 2a. 已关联 → 取出对应用户
		user, err := userRepo.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}

		// 被禁用的账户不允许通过第三方登录进入系统
		if user.Status != constants.UserStatusActive {
			return nil, fmt.Errorf("user account is disabled: %w", domain.ErrUserDisabled)
		}

		// 按 provider 最新资料刷新 nickname/avatar（email 不刷新：
		// provider email 变化可能撞上 (source, email) 唯一约束，登录路径不应因邮箱冲突而失败）。
		if err := u.refreshUserProfile(ctx, userRepo, user, profile); err != nil {
			return nil, err
		}
		return user, nil
	}

	// 2b. 未关联 → 创建独立用户 + 建立关联记录。
	// 不按 email 合并：oidc 用户与本地用户完全隔离，不同 provider 账号各自独立；
	// 同渠道（source）内 email 唯一由 (source, email) 复合唯一索引保证。
	user, err := u.createUserFromUserIdentity(ctx, userRepo, provider, profile)
	if err != nil {
		return nil, fmt.Errorf("create user from user identity profile: %w", err)
	}
	err = identityRepo.Upsert(ctx, &domain.UserIdentity{
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: profile.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert identity account: %w", err)
	}
	return user, nil
}

// createUserFromUserIdentity 基于第三方 profile 创建新 shadmin 用户。
// 第三方来源用户（provider:user）与本地用户（shadmin:user）的区别：
//   - source = provider
//   - 无本地密码（password = NULL），不可用密码登录
//   - email 直接采用 provider 返回值，可能为空（存 NULL）
//   - nickname/avatar 取自 provider，随登录刷新
//   - username 由 provider + subject 稳定派生，保证全局唯一且可读
func (u *userIdentityUsecase) createUserFromUserIdentity(ctx context.Context, userRepo domain.UserRepository, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	email := strings.TrimSpace(profile.Email)
	name := providerDisplayName(profile)

	username := buildOAuthUsername(provider, profile.UserID, name)

	user := &domain.User{
		Username: username,
		Nickname: name,
		Email:    email, // 可能为空 → 仓储层写入 NULL
		Avatar:   strings.TrimSpace(profile.AvatarURL),
		Source:   provider,
		Status:   constants.UserStatusActive,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user identity user: %w", err)
	}
	return user, nil
}

// providerDisplayName 取 provider 返回的可读名称：Name 优先，其次 NickName。
func providerDisplayName(profile domain.UserIdentityProfile) string {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.TrimSpace(profile.NickName)
	}
	return name
}

// refreshUserProfile 按 provider 最新资料刷新用户昵称与头像。
// email 不在此处刷新：provider email 变化可能撞上 (source, email) 唯一约束，
// 登录路径不应因邮箱冲突而失败（email 仅在建号时写入）。
func (u *userIdentityUsecase) refreshUserProfile(ctx context.Context, userRepo domain.UserRepository, user *domain.User, profile domain.UserIdentityProfile) error {
	user.Nickname = providerDisplayName(profile)
	user.Avatar = strings.TrimSpace(profile.AvatarURL)
	if err := userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("refresh user profile: %w", err)
	}
	return nil
}

// buildOAuthUsername 基于 provider + subject 稳定派生唯一且可读的用户名。
// subject 在 provider 内唯一，叠加 provider 前缀后全局唯一，无需依赖唯一冲突重试。
func buildOAuthUsername(provider, subject, name string) string {
	base := slugifyUsername(name)
	if base == "" {
		base = strings.ToLower(provider)
	}
	if len(base) > 16 {
		base = base[:16]
	}
	suffix := usernameSuffix(provider, subject)
	return fmt.Sprintf("%s_%s", base, suffix)
}

// slugifyUsername 保留字母/数字，其余转为空，用于生成安全的用户名基段。
func slugifyUsername(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// usernameSuffix 由 provider+subject 生成稳定的短哈希后缀，保证用户名唯一。
func usernameSuffix(provider, subject string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(provider) + ":" + subject))
	return hex.EncodeToString(sum[:])[:10]
}

// isUniqueViolation 粗略判定唯一约束冲突（跨 sqlite/postgres/mysql 文案差异）
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint") || strings.Contains(msg, "duplicate")
}
