package tokenservice

import (
	"fmt"
	"time"

	"shadmin/domain"
	"shadmin/internal/tokenutil"

	"github.com/golang-jwt/jwt/v5"
)

// Config 的令牌有效期单位为分钟。
type Config struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  int
	RefreshExpiry int
}

type TokenService struct {
	cfg Config
}

var _ domain.TokenIssuer = (*TokenService)(nil)

func NewTokenService(cfg Config) *TokenService {
	return &TokenService{cfg: cfg}
}

func (ts *TokenService) IssuePair(user *domain.User) (*domain.LoginResponse, error) {
	access, err := tokenutil.CreateAccessToken(user, ts.cfg.AccessSecret, ts.cfg.AccessExpiry)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	return ts.withRefresh(user, access)
}

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

func (ts *TokenService) ExtractAccessJTI(requestToken string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, ts.cfg.AccessSecret)
}

func (ts *TokenService) ExtractRefreshJTI(requestToken string) (string, time.Time, bool) {
	return tokenutil.ExtractJTIAndExpiry(requestToken, ts.cfg.RefreshSecret)
}

func expiryTime(exp *jwt.NumericDate) time.Time {
	if exp == nil {
		return time.Time{}
	}
	return exp.Time
}
