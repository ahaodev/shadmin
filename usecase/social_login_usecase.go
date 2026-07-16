package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/internal/tokenservice"

	"golang.org/x/crypto/bcrypt"
)

type socialLoginUsecase struct {
	userRepo           domain.UserRepository
	socialAccountRepo  domain.SocialAccountRepository
	tokenService       *tokenservice.TokenService
	accessTokenSecret  string
	refreshTokenSecret string
	accessTokenExpiry  int
	refreshTokenExpiry int
	contextTimeout     time.Duration
}

// NewSocialLoginUsecase 构造第三方登录用例。
// token 相关参数与 device_auth_usecase 保持一致：复用同一套 TokenService + env secrets，
// 不引入独立的令牌签发流程。
func NewSocialLoginUsecase(
	userRepo domain.UserRepository,
	socialAccountRepo domain.SocialAccountRepository,
	tokenService *tokenservice.TokenService,
	accessTokenSecret, refreshTokenSecret string,
	accessTokenExpiry, refreshTokenExpiry int,
	timeout time.Duration,
) domain.SocialLoginUsecase {
	return &socialLoginUsecase{
		userRepo:           userRepo,
		socialAccountRepo:  socialAccountRepo,
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
func (u *socialLoginUsecase) HandleCallback(ctx context.Context, provider string, profile *domain.SocialProfile) (*domain.SocialLoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required: %w", domain.ErrSocialAuthFailed)
	}
	if strings.TrimSpace(profile.UserID) == "" {
		return nil, fmt.Errorf("provider %s returned empty subject: %w", provider, domain.ErrSocialAuthFailed)
	}

	// 1. 先查第三方账号是否已绑定
	account, err := u.socialAccountRepo.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("find social account: %w", err)
	}

	var user *domain.User
	if account != nil {
		// 2a. 已绑定 → 直接取出对应 shadmin 用户
		user, err = u.userRepo.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("get bound user: %w", err)
		}
	} else if strings.TrimSpace(profile.Email) != "" {
		// 2b. 未绑定但 provider 返回了邮箱 → 尝试用邮箱匹配已有用户
		user, err = u.userRepo.GetByEmail(ctx, profile.Email)
		if err != nil && !ent.IsNotFound(err) {
			return nil, fmt.Errorf("get user by email: %w", err)
		}
		// ent NotFound 时 user 为 nil，下面走创建分支
	}

	// 3. 用户不存在 → 基于第三方 profile 创建新用户
	if user == nil {
		user, err = u.createUserFromSocial(ctx, provider, profile)
		if err != nil {
			return nil, fmt.Errorf("create user from social profile: %w", err)
		}
	}

	// 被禁用的账户不允许通过第三方登录继续进入系统
	if user.Status != domain.UserStatusActive {
		return nil, fmt.Errorf("账户未启用或已停用: %w", domain.ErrUserDisabled)
	}

	// 4. 绑定（或更新）第三方账号
	err = u.socialAccountRepo.Upsert(ctx, &domain.SocialAccount{
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: profile.UserID,
		Email:           strings.TrimSpace(profile.Email),
		Name:            strings.TrimSpace(profile.Name),
		AvatarURL:       strings.TrimSpace(profile.AvatarURL),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert social account: %w", err)
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

	return &domain.SocialLoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// createUserFromSocial 基于第三方 profile 创建新 shadmin 用户。
//   - username：优先使用 email；无 email 时用 <provider>_<subject 前 16 位>
//   - email：有则用；无则用 <provider>_<subject>@social.local 占位（字段唯一且必填）
//   - password：随机 32 字节再 bcrypt 哈希，使该账户无法用密码登录，只能走 OAuth
//   - status：active，不分配角色（普通用户）
func (u *socialLoginUsecase) createUserFromSocial(ctx context.Context, provider string, profile *domain.SocialProfile) (*domain.User, error) {
	email := strings.TrimSpace(profile.Email)
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.TrimSpace(profile.NickName)
	}

	username := email
	if username == "" {
		// 截断 subject，避免 username 超过 32 字符上限
		sub := profile.UserID
		if len(sub) > 16 {
			sub = sub[:16]
		}
		username = fmt.Sprintf("%s_%s", provider, sub)
	}
	if email == "" {
		// 占位邮箱：保证唯一且符合字段必填/唯一约束
		email = fmt.Sprintf("%s_%s@social.local", provider, profile.UserID)
	}

	// 防止 username/email 偶发碰撞：撞了就追加短随机后缀重试一次
	user, err := u.tryCreateUser(ctx, username, email, name, profile.AvatarURL)
	if err != nil && isUniqueViolation(err) {
		suffix, _ := randomSuffix(6)
		username = truncate(username, 32-len(suffix)-1) + "_" + suffix
		user, err = u.tryCreateUser(ctx, username, email, name, profile.AvatarURL)
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (u *socialLoginUsecase) tryCreateUser(ctx context.Context, username, email, name, avatarURL string) (*domain.User, error) {
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
		Avatar:   strings.TrimSpace(avatarURL),
		Password: string(hashed),
		Status:   domain.UserStatusActive,
	}
	_ = name // 当前 User 实体无独立 displayName 字段，name 暂不落库

	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create social user: %w", err)
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
	return s[:max]
}

// isUniqueViolation 粗略判定唯一约束冲突（跨 sqlite/postgres/mysql 文案差异）
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint") || strings.Contains(msg, "duplicate")
}
