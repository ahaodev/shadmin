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
)

type userIdentityUsecase struct {
	uow                domain.UnitOfWork
	userRepository     domain.UserRepository
	roleRepository     domain.RoleRepository
	identityRepository domain.UserIdentityRepository
	tokenService       domain.TokenIssuer
	codeStore          domain.UserIdentityCodeStore
	loginLogUsecase    domain.LoginLogUseCase
	contextTimeout     time.Duration
}

func NewUserIdentityUsecase(
	uow domain.UnitOfWork,
	userRepository domain.UserRepository,
	roleRepository domain.RoleRepository,
	identityRepository domain.UserIdentityRepository,
	tokenService domain.TokenIssuer,
	codeStore domain.UserIdentityCodeStore,
	loginLogUsecase domain.LoginLogUseCase,
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
		contextTimeout:     timeout,
	}
}

func (u *userIdentityUsecase) HandleCallback(ctx context.Context, provider string, profile domain.UserIdentityProfile, meta domain.LoginMeta) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	result, err := u.issueTokens(ctx, provider, profile)
	if err != nil {
		reason := "第三方登录处理失败"
		if errors.Is(err, domain.ErrUserDisabled) {
			reason = "账户已停用或未启用"
		}
		u.recordLoginLog(ctx, meta, provider, profile.Email, domain.StatusFailed, reason)
		return "", err
	}

	u.recordLoginLog(ctx, meta, provider, profile.Email, domain.StatusSuccess, "")

	code, err := u.codeStore.Put(ctx, result)
	if err != nil {
		return "", fmt.Errorf("store identity login code: %w", err)
	}
	return code, nil
}

func (u *userIdentityUsecase) RecordFailure(ctx context.Context, provider string, meta domain.LoginMeta, reason string) {
	u.recordLoginLog(ctx, meta, provider, "", domain.StatusFailed, reason)
}

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

func (u *userIdentityUsecase) recordLoginLog(ctx context.Context, meta domain.LoginMeta, provider, email, status, failureReason string) {
	writeLoginLog(ctx, u.loginLogUsecase, meta, provider, status, failureReason, email)
}

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

	return u.tokenService.IssuePairWithIdentity(user, provider, profile.UserID)
}

// resolveOrCreateUser 遇到并发建号的唯一约束冲突时，在新事务中重试一次。
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
	account, err := u.identityRepository.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find identity account: %w", err)
	}

	if account != nil {
		user, err := u.userRepository.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}

		if user.Status != domain.UserStatusActive {
			return nil, fmt.Errorf("user account is disabled: %w", domain.ErrUserDisabled)
		}

		if err := u.refreshUserProfile(ctx, user, profile); err != nil {
			return nil, err
		}
		return user, nil
	}

	// 不按邮箱合并账号，各 provider 身份独立。
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

func (u *userIdentityUsecase) createUserFromUserIdentity(ctx context.Context, provider string, profile domain.UserIdentityProfile) (*domain.User, error) {
	viewerRole, err := u.roleRepository.GetByName(ctx, domain.RoleNameViewer)
	if err != nil {
		return nil, fmt.Errorf("get default identity role: %w", err)
	}
	if viewerRole.Status != domain.StatusActive {
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
		Status:   domain.UserStatusActive,
	}

	if err := u.userRepository.CreateWithRoles(ctx, user, []string{viewerRole.ID}); err != nil {
		return nil, fmt.Errorf("create user identity user with default role: %w", err)
	}
	return user, nil
}

func providerDisplayName(profile domain.UserIdentityProfile) string {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.TrimSpace(profile.NickName)
	}
	return name
}

// refreshUserProfile 不更新邮箱，避免邮箱唯一约束冲突阻断登录。
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

func slugifyUsername(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func usernameSuffix(provider, subject string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(provider) + ":" + subject))
	return hex.EncodeToString(sum[:])[:15]
}
