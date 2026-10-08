package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/internal/constants"
)

type userIdentityUsecase struct {
	uow                domain.UnitOfWork
	userRepository     domain.UserRepository
	roleRepository     domain.RoleRepository
	identityRepository domain.UserIdentityRepository
	tokenService       domain.TokenIssuer
	codeStore          domain.UserIdentityCodeStore
	loginLogUsecase    domain.LoginLogUseCase
	accessTokenSecret  string
	refreshTokenSecret string
	accessTokenExpiry  int
	refreshTokenExpiry int
	contextTimeout     time.Duration
}

// NewUserIdentityUsecase 构造第三方登录用例。绑定事务由 uow 统一开启，不引入独立的令牌签发流程。
func NewUserIdentityUsecase(
	uow domain.UnitOfWork,
	userRepository domain.UserRepository,
	roleRepository domain.RoleRepository,
	identityRepository domain.UserIdentityRepository,
	tokenService domain.TokenIssuer,
	codeStore domain.UserIdentityCodeStore,
	loginLogUsecase domain.LoginLogUseCase,
	accessTokenSecret, refreshTokenSecret string,
	accessTokenExpiry, refreshTokenExpiry int,
	timeout time.Duration,
) domain.UserIdentityUsecase {
	return &userIdentityUsecase{
		uow:                uow,
		userRepository:     userRepository,
		roleRepository:     roleRepository,
		identityRepository: identityRepository,
		tokenService:       tokenService,
		codeStore:          codeStore,
		loginLogUsecase:    loginLogUsecase,
		accessTokenSecret:  accessTokenSecret,
		refreshTokenSecret: refreshTokenSecret,
		accessTokenExpiry:  accessTokenExpiry,
		refreshTokenExpiry: refreshTokenExpiry,
		contextTimeout:     timeout,
	}
}

// HandleCallback 处理 provider 回调：解析第三方 profile，查找/创建用户，签发令牌对，
// 记录登录日志，并把令牌对存入一次性 code 返回给 controller。
func (u *userIdentityUsecase) HandleCallback(ctx context.Context, provider string, profile domain.UserIdentityProfile, meta domain.LoginMeta) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	result, err := u.issueTokens(ctx, provider, profile)
	if err != nil {
		reason := "第三方登录处理失败"
		if errors.Is(err, domain.ErrUserDisabled) {
			reason = "账户已停用或未启用"
		}
		u.recordLoginLog(ctx, meta, provider, profile.Email, constants.StatusFailed, reason)
		return "", err
	}

	u.recordLoginLog(ctx, meta, provider, profile.Email, constants.StatusSuccess, "")

	code, err := u.codeStore.Put(ctx, result)
	if err != nil {
		return "", fmt.Errorf("store identity login code: %w", err)
	}
	return code, nil
}

// RecordFailure 记录 provider 认证阶段的失败（此时尚无第三方资料，邮箱留空）。
func (u *userIdentityUsecase) RecordFailure(ctx context.Context, provider string, meta domain.LoginMeta, reason string) {
	u.recordLoginLog(ctx, meta, provider, "", constants.StatusFailed, reason)
}

// Exchange 一次性消费 code 换取令牌对；成功即删除，避免重放。
func (u *userIdentityUsecase) Exchange(ctx context.Context, code string) (*domain.UserIdentityResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	result, err := u.codeStore.Consume(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("consume identity login code: %w", err)
	}
	if result == nil {
		return nil, domain.ErrUserIdentityCodeInvalid
	}
	return result, nil
}

// recordLoginLog 第三方登录日志，来源记为 provider 名（github/google）。
func (u *userIdentityUsecase) recordLoginLog(ctx context.Context, meta domain.LoginMeta, provider, email, status, failureReason string) {
	writeLoginLog(ctx, u.loginLogUsecase, meta, provider, status, failureReason, email)
}

// issueTokens 绑定或创建用户后，复用既有 TokenService 签发 JWT 令牌对（sub = provider:provider_subject）。
func (u *userIdentityUsecase) issueTokens(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.UserIdentityResult, error) {
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

	accessToken, err := u.tokenService.CreateAccessTokenWithIdentity(user, u.accessTokenSecret, u.accessTokenExpiry, provider, profile.UserID, provider)
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
// 并发首次登录可能同时建号，唯一约束冲突时在新事务中重试一次。
func (u *userIdentityUsecase) resolveOrCreateUser(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	var lastErr error
	for range 2 {
		var user *domain.User
		err := u.uow.Do(ctx, func(txCtx context.Context) error {
			var err error
			user, err = u.resolveOrCreateUserForIdentity(txCtx, provider, profile)
			return err
		})
		if err == nil {
			return user, nil
		}
		lastErr = err
		if !errors.Is(err, domain.ErrUserIdentityConflict) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (u *userIdentityUsecase) resolveOrCreateUserForIdentity(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	// 1. 先查该 (provider, provider_subject) 是否已关联
	account, err := u.identityRepository.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find identity account: %w", err)
	}

	if account != nil {
		// 2a. 已关联 → 取出对应用户
		user, err := u.userRepository.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}

		// 被禁用的账户不允许通过第三方登录进入系统
		if user.Status != constants.UserStatusActive {
			return nil, fmt.Errorf("user account is disabled: %w", domain.ErrUserDisabled)
		}

		// 按 provider 最新资料刷新 nickname/avatar（email 不刷新：
		// provider email 变化可能撞上 (source, email) 唯一约束，登录路径不应因邮箱冲突而失败）。
		if err := u.refreshUserProfile(ctx, user, profile); err != nil {
			return nil, err
		}
		return user, nil
	}

	// 2b. 未关联 → 创建独立用户 + 建立关联记录。
	// 不按 email 合并：oidc 用户与本地用户完全隔离，不同 provider 账号各自独立；
	// 同渠道（source）内 email 唯一由 (source, email) 复合唯一索引保证。
	user, err := u.createUserFromUserIdentity(ctx, provider, profile)
	if err != nil {
		return nil, fmt.Errorf("create user from user identity profile: %w", err)
	}
	err = u.identityRepository.Upsert(ctx, &domain.UserIdentity{
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: profile.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert identity account: %w", err)
	}
	return user, nil
}

// createUserFromUserIdentity 基于第三方资料创建独立用户，绑定启用的 viewer 角色，
// 不设置本地密码，也不按邮箱合并账号。
func (u *userIdentityUsecase) createUserFromUserIdentity(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	viewerRole, err := u.roleRepository.GetByName(ctx, domain.RoleNameViewer)
	if err != nil {
		return nil, fmt.Errorf("get default identity role: %w", err)
	}
	if viewerRole.Status != constants.StatusActive {
		return nil, fmt.Errorf("default identity role %q is inactive", domain.RoleNameViewer)
	}

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

	if err := u.userRepository.CreateWithRoles(ctx, user, []string{viewerRole.ID}); err != nil {
		return nil, fmt.Errorf("create user identity user with default role: %w", err)
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
func (u *userIdentityUsecase) refreshUserProfile(ctx context.Context, user *domain.User, profile domain.UserIdentityProfile) error {
	nickname := providerDisplayName(profile)
	avatar := strings.TrimSpace(profile.AvatarURL)
	if err := u.userRepository.UpdateIdentityProfile(ctx, user.ID, nickname, avatar); err != nil {
		return fmt.Errorf("refresh user profile: %w", err)
	}
	user.Nickname = nickname
	user.Avatar = avatar
	return nil
}

// buildOAuthUsername 基于 provider + subject 稳定派生可读用户名，并通过哈希后缀降低冲突概率。
func buildOAuthUsername(provider, subject, name string) string {
	base := slugifyUsername(name)
	if base == "" {
		base = strings.ToLower(provider)
	}
	if runes := []rune(base); len(runes) > 16 {
		base = string(runes[:16])
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

// usernameSuffix 由 provider+subject 生成稳定的哈希后缀，降低用户名冲突概率。
func usernameSuffix(provider, subject string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(provider) + ":" + subject))
	return hex.EncodeToString(sum[:])[:15]
}
