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

// hmacKeyFunc 只接受 HS256。
func hmacKeyFunc(secret string) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}
}

func CreateAccessToken(user *domain.User, secret string, expiry int) (accessToken string, err error) {
	return CreateAccessTokenWithIdentity(user, secret, expiry, "shadmin", user.ID)
}

func CreateAccessTokenWithIdentity(user *domain.User, secret string, expiry int, provider, providerSubject string) (accessToken string, err error) {
	exp := jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry)))

	// shamdin users    shadmin:user_id
	// OIDC Provider    provider:provider_subject
	subject := provider + ":" + providerSubject

	claims := &domain.JwtCustomClaims{
		Name:    user.Username,
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
		Roles:   user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "shadmin",
			ExpiresAt: exp,
			ID:        xid.New().String(), // jti：登出黑名单的键
			Subject:   subject,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return t, err
}

func CreateRefreshToken(user *domain.User, secret string, expiry int) (refreshToken string, err error) {
	exp := jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(expiry)))
	claimsRefresh := &domain.JwtCustomRefreshClaims{
		ID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: exp,
			ID:        xid.New().String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsRefresh)
	rt, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return rt, err
}

// ParseAccessClaims 校验签名与 exp，并解析 access token 的全部 claims。
func ParseAccessClaims(requestToken string, secret string) (*domain.JwtCustomClaims, error) {
	claims := new(domain.JwtCustomClaims)
	token, err := jwt.ParseWithClaims(requestToken, claims, hmacKeyFunc(secret))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ParseRefreshClaims 校验签名与 exp，并解析 refresh token 的全部 claims。
func ParseRefreshClaims(requestToken string, secret string) (*domain.JwtCustomRefreshClaims, error) {
	claims := new(domain.JwtCustomRefreshClaims)
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
