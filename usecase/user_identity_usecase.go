package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/internal/tokenservice"

	"golang.org/x/crypto/bcrypt"
)

type userIdentityUsecase struct {
	userRepo           domain.UserRepository
	userIdentityRepo   domain.UserIdentityRepository
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
	userRepo domain.UserRepository,
	userIdentityRepo domain.UserIdentityRepository,
	tokenService *tokenservice.TokenService,
	accessTokenSecret, refreshTokenSecret string,
	accessTokenExpiry, refreshTokenExpiry int,
	timeout time.Duration,
) domain.UserIdentityUsecase {
	return &userIdentityUsecase{
		userRepo:           userRepo,
		userIdentityRepo:   userIdentityRepo,
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

	// 1. 先查该 (provider, provider_subject) 是否已绑定
	account, err := u.userIdentityRepo.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find identity account: %w", err)
	}

	var user *domain.User
	if account != nil {
		// 2a. 已绑定 → 直接取出对应 shadmin 用户
		user, err = u.userRepo.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}
	} else {
		// 2b. 未绑定 → 检查相同 email 的用户是否已存在
		// 如果存在，则绑定到同一用户；如果不存在，创建新用户
		email := strings.TrimSpace(profile.Email)
		if email != "" {
			user, err = u.userRepo.GetByEmail(ctx, email)
			if err != nil && !isNotFound(err) {
				return nil, fmt.Errorf("find user by email: %w", err)
			}
		}

		// 3. 用户不存在 → 基于第三方 profile 创建新用户
		if user == nil {
			user, err = u.createUserFromUserIdentity(ctx, provider, profile)
			if err != nil {
				return nil, fmt.Errorf("create user from user identity profile: %w", err)
			}
		}
	}

	// 被禁用的账户不允许通过第三方登录继续进入系统
	if user.Status != domain.UserStatusActive {
		return nil, fmt.Errorf("user account is disabled: %w", domain.ErrUserDisabled)
	}

	// 4. 绑定（或更新）第三方账号
	err = u.userIdentityRepo.Upsert(ctx, &domain.UserIdentity{
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

	// 5. 复用既有 TokenService 签发 JWT 令牌对
	accessToken, err := u.tokenService.CreateAccessToken(user, u.accessTokenSecret, u.accessTokenExpiry)
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
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		User:              user,
		ProviderAvatarURL: strings.TrimSpace(profile.AvatarURL),
	}, nil
}

// createUserFromUserIdentity 基于第三方 profile 创建新 shadmin 用户。
func (u *userIdentityUsecase) createUserFromUserIdentity(ctx context.Context, provider string, profile *domain.UserIdentityProfile) (*domain.User, error) {
	email := strings.TrimSpace(profile.Email)
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.TrimSpace(profile.NickName)
	}

	username := email
	if username == "" {
		sub := profile.UserID
		if len(sub) > 16 {
			sub = sub[:16]
		}
		username = fmt.Sprintf("%s_%s", provider, sub)
	}
	if email == "" {
		email = fmt.Sprintf("%s_%s@id.local", provider, profile.UserID)
	}

	// 碰撞重试：username 或 email 唯一约束冲突时
	//  - username 改为追加随机后缀
	//  - email 若为真实邮箱（可能与其他 provider 的同邮箱用户冲突），
	//    换用 <provider>_<subject>@id.local 占位，确保跨 provider 同邮箱各自独立
	user, err := u.tryCreateUser(ctx, username, email, name)
	if err != nil && isUniqueViolation(err) {
		suffix, suffixErr := randomSuffix(6)
		if suffixErr != nil {
			return nil, fmt.Errorf("generate username suffix: %w", suffixErr)
		}
		username = truncate(username, 32-len(suffix)-1) + "_" + suffix
		placeholderEmail := fmt.Sprintf("%s_%s@id.local", provider, profile.UserID)
		user, err = u.tryCreateUser(ctx, username, placeholderEmail, name)
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (u *userIdentityUsecase) tryCreateUser(ctx context.Context, username, email, name string) (*domain.User, error) {
	rawPwd, err := randomSuffix(32)
	if err != nil {
		return nil, fmt.Errorf("generate random password: %w", err)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(rawPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash random password: %w", err)
	}

	user := &domain.User{
		Username: username,
		Email:    email,
		Password: string(hashed),
		Status:   domain.UserStatusActive,
	}
	_ = name // 当前 User 实体无独立 displayName 字段，name 暂不落库

	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user identity user: %w", err)
	}
	return user, nil
}

// randomSuffix 生成 url-safe 随机字符串
func randomSuffix(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
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
