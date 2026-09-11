package tokenservice

import (
	"shadmin/domain"
	"shadmin/internal/tokenutil"
	"time"
)

// TokenService 认证令牌服务
type TokenService struct{}

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
func (ts *TokenService) ParseAccessClaims(requestToken string, secret string) (*domain.JwtCustomClaims, error) {
	return tokenutil.ParseAccessClaims(requestToken, secret)
}

// ParseRefreshClaims 校验 refresh token 签名并解析全部 claims。
func (ts *TokenService) ParseRefreshClaims(requestToken string, secret string) (*domain.JwtCustomRefreshClaims, error) {
	return tokenutil.ParseRefreshClaims(requestToken, secret)
}

// ExtractJTIAndExpiry 提取 jti 与过期时间（用于服务端登出黑名单）
func (ts *TokenService) ExtractJTIAndExpiry(requestToken string, secret string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, secret)
}
