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
// 绑定第三方账号，复用既有 JWT 体系签发令牌对。
// 支持多 provider 绑定到同一用户：相同 email 的不同 provider 自动合并到同一用户。
func (u *userIdentityUsecase) HandleCallback(ctx context.Context, provider string, profile *domain.UserIdentityProfile) (*domain.UserIdentityResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required: %w", domain.ErrUserIdentityAuthFailed)
	}
	if strings.TrimSpace(profile.UserID) == "" {
		return nil, fmt.Errorf("provider %s returned empty subject: %w", provider, domain.ErrUserIdentityAuthFailed)
	}

	user, _, err := u.findOrCreateAndBindUser(ctx, provider, profile)
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

	// 不再把密码哈希回传给前端
	user.Password = ""

	return &domain.UserIdentityResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (u *userIdentityUsecase) findOrCreateAndBindUser(ctx context.Context, provider string, profile *domain.UserIdentityProfile) (*domain.User, *domain.UserIdentity, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		user, err := u.findOrCreateAndBindUserOnce(ctx, provider, profile)
		if err == nil {
			identity, err := u.identityRepository.FindByProviderAndSubject(ctx, provider, profile.UserID)
			if err != nil {
				return nil, nil, fmt.Errorf("find identity account after binding: %w", err)
			}
			if identity == nil {
				return nil, nil, fmt.Errorf("identity account missing after binding: %w", domain.ErrUserIdentityAuthFailed)
			}
			return user, identity, nil
		}
		lastErr = err
		if !isUniqueViolation(err) {
			return nil, nil, err
		}
	}
	return nil, nil, lastErr
}

func (u *userIdentityUsecase) findOrCreateAndBindUserOnce(ctx context.Context, provider string, profile *domain.UserIdentityProfile) (*domain.User, error) {
	return u.identityRepository.WithUserBindingTx(ctx, func(txCtx context.Context, userRepo domain.UserRepository, identityRepo domain.UserIdentityRepository) (*domain.User, error) {
		return u.findOrCreateUserForIdentity(txCtx, userRepo, identityRepo, provider, profile)
	})
}

func (u *userIdentityUsecase) findOrCreateUserForIdentity(
	ctx context.Context,
	userRepo domain.UserRepository,
	identityRepo domain.UserIdentityRepository,
	provider string,
	profile *domain.UserIdentityProfile,
) (*domain.User, error) {
	// 1. 先查该 (provider, provider_subject) 是否已绑定
	account, err := identityRepo.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find identity account: %w", err)
	}

	var user *domain.User
	if account != nil {
		// 2a. 已绑定 → 直接取出对应 shadmin 用户
		user, err = userRepo.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}
	} else {
		// 2b. 未绑定 → 检查相同 email 的用户是否已存在
		// 如果存在，则绑定到同一用户；如果不存在，创建新用户
		email := strings.TrimSpace(profile.Email)
		if email != "" {
			user, err = userRepo.GetByEmail(ctx, email)
			if err != nil && !isNotFound(err) {
				return nil, fmt.Errorf("find user by email: %w", err)
			}
		}

		// 3. 用户不存在 → 基于第三方 profile 创建新用户
		if user == nil {
			user, err = u.createUserFromUserIdentity(ctx, userRepo, provider, profile)
			if err != nil {
				return nil, fmt.Errorf("create user from user identity profile: %w", err)
			}
		}
	}

	// 被禁用的账户不允许通过第三方登录继续进入系统
	if user.Status != constants.UserStatusActive {
		return nil, fmt.Errorf("user account is disabled: %w", domain.ErrUserDisabled)
	}

	// 4. 绑定（或更新）第三方账号
	err = identityRepo.Upsert(ctx, &domain.UserIdentity{
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: profile.UserID,
		Email:           strings.TrimSpace(profile.Email),
		Name:            strings.TrimSpace(profile.Name),
		AvatarURL:       strings.TrimSpace(profile.AvatarURL),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert identity account: %w", err)
	}
	return user, nil
}

// createUserFromUserIdentity 基于第三方 profile 创建新 shadmin 用户。
// 第三方来源用户（provider:user）与本地用户（shadmin:user）的区别：
//   - source = oauth
//   - 无本地密码（password = NULL），不可用密码登录
//   - email 直接采用 provider 返回值，可能为空（存 NULL），不再伪造 @id.local
//   - username 由 provider + subject 稳定派生，保证全局唯一且可读
func (u *userIdentityUsecase) createUserFromUserIdentity(ctx context.Context, userRepo domain.UserRepository, provider string, profile *domain.UserIdentityProfile) (*domain.User, error) {
	email := strings.TrimSpace(profile.Email)
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.TrimSpace(profile.NickName)
	}

	username := buildOAuthUsername(provider, profile.UserID, name)

	user := &domain.User{
		Username: username,
		Nickname: name,
		Email:    email, // 可能为空 → 仓储层写入 NULL
		Source:   constants.UserSourceOAuth,
		Status:   constants.UserStatusActive,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user identity user: %w", err)
	}
	return user, nil
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

// isNotFound 判定是否为记录不存在的错误
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "no rows")
}
