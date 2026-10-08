package tokenservice

import (
	"shadmin/domain"
	"shadmin/internal/tokenutil"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenService 认证令牌服务，实现 domain.TokenIssuer。
type TokenService struct{}

var _ domain.TokenIssuer = (*TokenService)(nil)

// NewTokenService 创建新的令牌服务实例
func NewTokenService() *TokenService {
	return &TokenService{}
}

// CreateAccessToken 创建访问令牌
func (ts *TokenService) CreateAccessToken(user *domain.User, secret string, expiry int) (string, error) {
	return tokenutil.CreateAccessToken(user, secret, expiry)
}

// CreateAccessTokenWithIdentity 创建携带第三方身份信息的访问令牌。
func (ts *TokenService) CreateAccessTokenWithIdentity(user *domain.User, secret string, expiry int, provider, providerSubject, source string) (string, error) {
	return tokenutil.CreateAccessTokenWithIdentity(user, secret, expiry, provider, providerSubject)
}

// CreateRefreshToken 创建刷新令牌
func (ts *TokenService) CreateRefreshToken(user *domain.User, secret string, expiry int) (string, error) {
	return tokenutil.CreateRefreshToken(user, secret, expiry)
}

// ParseAccessClaims 校验 access token 签名并解析全部 claims。
func (ts *TokenService) ParseAccessClaims(requestToken string, secret string) (*domain.TokenClaims, error) {
	c, err := tokenutil.ParseAccessClaims(requestToken, secret)
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
func (ts *TokenService) ParseRefreshClaims(requestToken string, secret string) (*domain.TokenClaims, error) {
	c, err := tokenutil.ParseRefreshClaims(requestToken, secret)
	if err != nil {
		return nil, err
	}
	return &domain.TokenClaims{
		ID:        c.ID,
		JTI:       c.JTI(),
		ExpiresAt: expiryTime(c.ExpiresAt),
	}, nil
}

// ExtractJTIAndExpiry 提取 jti 与过期时间（用于服务端登出黑名单）
func (ts *TokenService) ExtractJTIAndExpiry(requestToken string, secret string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, secret)
}

// expiryTime 把可选的 JWT exp 转为 time.Time；缺省时返回零值。
func expiryTime(exp *jwt.NumericDate) time.Time {
	if exp == nil {
		return time.Time{}
	}
	return exp.Time
}
