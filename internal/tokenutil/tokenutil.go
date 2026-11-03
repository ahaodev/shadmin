package tokenutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"shadmin/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
)

// 校验签发者，防止 refresh token 被当作 access token。
const accessTokenIssuer = "shadmin"

type AccessClaims struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	IsAdmin bool     `json:"is_admin"`
	Roles   []string `json:"roles"`
	jwt.RegisteredClaims
}

// JTI 区分令牌 ID 与外层的用户 ID。
func (c *AccessClaims) JTI() string { return c.RegisteredClaims.ID }

type RefreshClaims struct {
	ID string `json:"id"`
	jwt.RegisteredClaims
}

// JTI 区分令牌 ID 与外层的用户 ID。
func (c *RefreshClaims) JTI() string { return c.RegisteredClaims.ID }

func hmacKeyFunc(secret string) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}
}

func CreateAccessToken(user *domain.User, secret string, expiry int) (string, error) {
	return CreateAccessTokenWithIdentity(user, secret, expiry, accessTokenIssuer, user.ID)
}

func CreateAccessTokenWithIdentity(user *domain.User, secret string, expiry int, provider, providerSubject string) (string, error) {
	exp := jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry)))

	subject := provider + ":" + providerSubject

	claims := &AccessClaims{
		Name:    user.Username,
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
		Roles:   user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			ExpiresAt: exp,
			ID:        xid.New().String(), // jti：登出黑名单的键
			Subject:   subject,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func CreateRefreshToken(user *domain.User, secret string, expiry int) (string, error) {
	exp := jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry)))
	claimsRefresh := &RefreshClaims{
		ID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: exp,
			ID:        xid.New().String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsRefresh)
	return token.SignedString([]byte(secret))
}

// ParseAccessClaims 校验签名、有效期和签发者。
func ParseAccessClaims(requestToken string, secret string) (*AccessClaims, error) {
	claims := new(AccessClaims)
	token, err := jwt.ParseWithClaims(requestToken, claims, hmacKeyFunc(secret), jwt.WithIssuer(accessTokenIssuer))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ParseRefreshClaims 校验签名和有效期。
func ParseRefreshClaims(requestToken string, secret string) (*RefreshClaims, error) {
	claims := new(RefreshClaims)
	token, err := jwt.ParseWithClaims(requestToken, claims, hmacKeyFunc(secret))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ExtractJTIAndExpiry 解析出 jti 与过期时间，供登出黑名单使用；ok=false 表示令牌不可用或无 exp。
func ExtractJTIAndExpiry(requestToken string, secret string) (jti string, expiresAt time.Time, ok bool) {
	token, err := jwt.Parse(requestToken, hmacKeyFunc(secret))
	if err != nil || !token.Valid {
		return "", time.Time{}, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", time.Time{}, false
	}
	jti, _ = claims["jti"].(string)
	if expClaim, exists := claims["exp"]; exists {
		switch v := expClaim.(type) {
		case float64:
			expiresAt = time.Unix(int64(v), 0)
		case int64:
			expiresAt = time.Unix(v, 0)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				expiresAt = time.Unix(n, 0)
			}
		}
	}
	if expiresAt.IsZero() {
		return jti, time.Time{}, false
	}
	return jti, expiresAt, true
}
