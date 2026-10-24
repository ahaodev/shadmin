package tokenservice

import (
	"fmt"
	"time"

	"shadmin/domain"
	"shadmin/internal/tokenutil"

	"github.com/golang-jwt/jwt/v5"
)

// Config 是令牌服务的签名配置。过期时间单位为分钟。
type Config struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  int
	RefreshExpiry int
}

// TokenService 认证令牌服务，实现 domain.TokenIssuer。
type TokenService struct {
	cfg Config
}

var _ domain.TokenIssuer = (*TokenService)(nil)

// NewTokenService 创建令牌服务实例。
func NewTokenService(cfg Config) *TokenService {
	return &TokenService{cfg: cfg}
}

// IssuePair 签发 access/refresh 令牌对。
func (ts *TokenService) IssuePair(user *domain.User) (*domain.LoginResponse, error) {
	access, err := tokenutil.CreateAccessToken(user, ts.cfg.AccessSecret, ts.cfg.AccessExpiry)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	return ts.withRefresh(user, access)
}

// IssuePairWithIdentity 签发携带第三方身份信息的令牌对。
func (ts *TokenService) IssuePairWithIdentity(user *domain.User, provider, providerSubject string) (*domain.LoginResponse, error) {
	access, err := tokenutil.CreateAccessTokenWithIdentity(user, ts.cfg.AccessSecret, ts.cfg.AccessExpiry, provider, providerSubject)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	return ts.withRefresh(user, access)
}

func (ts *TokenService) withRefresh(user *domain.User, access string) (*domain.LoginResponse, error) {
	refresh, err := tokenutil.CreateRefreshToken(user, ts.cfg.RefreshSecret, ts.cfg.RefreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}
	return &domain.LoginResponse{AccessToken: access, RefreshToken: refresh}, nil
}

// ParseAccessClaims 校验 access token 签名并解析全部 claims。
func (ts *TokenService) ParseAccessClaims(requestToken string) (*domain.TokenClaims, error) {
	c, err := tokenutil.ParseAccessClaims(requestToken, ts.cfg.AccessSecret)
	if err != nil {
		return nil, err
	}
	return &domain.TokenClaims{
		ID:        c.ID,
		Name:      c.Name,
		Email:     c.Email,
		IsAdmin:   c.IsAdmin,
		Roles:     c.Roles,
		Subject:   c.Subject,
		JTI:       c.JTI(),
		ExpiresAt: expiryTime(c.ExpiresAt),
	}, nil
}

// ParseRefreshClaims 校验 refresh token 签名并解析全部 claims。
func (ts *TokenService) ParseRefreshClaims(requestToken string) (*domain.TokenClaims, error) {
	c, err := tokenutil.ParseRefreshClaims(requestToken, ts.cfg.RefreshSecret)
	if err != nil {
		return nil, err
	}
	return &domain.TokenClaims{
		ID:        c.ID,
		JTI:       c.JTI(),
		ExpiresAt: expiryTime(c.ExpiresAt),
	}, nil
}

// ExtractAccessJTI 按 access 密钥提取 jti 与过期时间（用于服务端登出黑名单）。
func (ts *TokenService) ExtractAccessJTI(requestToken string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, ts.cfg.AccessSecret)
}

// ExtractRefreshJTI 按 refresh 密钥提取 jti 与过期时间。
func (ts *TokenService) ExtractRefreshJTI(requestToken string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, ts.cfg.RefreshSecret)
}

// expiryTime 把可选的 JWT exp 转为 time.Time；缺省时返回零值。
func expiryTime(exp *jwt.NumericDate) time.Time {
	if exp == nil {
		return time.Time{}
	}
	return exp.Time
}
