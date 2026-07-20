package tokenutil

import (
	"encoding/json"
	"fmt"
	"time"

	"shadmin/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
)

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
			ID:        xid.New().String(), // JTI，用于服务端登出黑名单
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
			ID:        xid.New().String(), // JTI
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsRefresh)
	rt, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return rt, err
}

func IsAuthorized(requestToken string, secret string) (bool, error) {
	_, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func parseTokenClaims(requestToken string, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// ExtractJTI 从 token 的 jti 声明中提取唯一标识，用于服务端登出黑名单。
// 老 token 无 jti 时返回空串（中间件视为不在黑名单，向后兼容）。
func ExtractJTI(requestToken string, secret string) (string, error) {
	token, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	jti, _ := claims["jti"].(string)
	return jti, nil
}

// ExtractJTIAndExpiry 一次解析取出 jti 与过期时间，专供登出黑名单使用。
// jti 为空串表示老令牌无 jti；ok=false 表示令牌不可解析或无 exp。
func ExtractJTIAndExpiry(requestToken string, secret string) (jti string, expiresAt time.Time, ok bool) {
	token, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
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

func ExtractIDFromToken(requestToken string, secret string) (string, error) {
	token, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok && !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	return claims["id"].(string), nil
}

// ExtractEmailFromToken 从token中提取用户邮箱
func ExtractEmailFromToken(requestToken string, secret string) (string, error) {
	token, err := jwt.Parse(requestToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok && !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	email, exists := claims["email"]
	if !exists {
		return "", nil // 邮箱不存在时返回空字符串
	}

	emailStr, ok := email.(string)
	if !ok {
		return "", fmt.Errorf("email claim is not a string")
	}

	return emailStr, nil
}

// ExtractIsAdminFromToken 从token中提取用户是否为管理员
func ExtractIsAdminFromToken(requestToken string, secret string) (bool, error) {
	claims, err := parseTokenClaims(requestToken, secret)
	if err != nil {
		return false, err
	}

	isAdmin, exists := claims["is_admin"]
	if !exists {
		return false, nil // is_admin不存在时返回false
	}

	isAdminBool, ok := isAdmin.(bool)
	if !ok {
		return false, fmt.Errorf("is_admin claim is not a boolean")
	}

	return isAdminBool, nil
}

// ExtractAllClaimsFromToken 从token中提取所有自定义claims
func ExtractAllClaimsFromToken(requestToken string, secret string) (*domain.JwtCustomClaims, error) {
	claims, err := parseTokenClaims(requestToken, secret)
	if err != nil {
		return nil, err
	}

	// 安全地提取各个字段
	customClaims := &domain.JwtCustomClaims{}

	if id, exists := claims["id"]; exists {
		if idStr, ok := id.(string); ok {
			customClaims.ID = idStr
		}
	}

	if name, exists := claims["name"]; exists {
		if nameStr, ok := name.(string); ok {
			customClaims.Name = nameStr
		}
	}

	if email, exists := claims["email"]; exists {
		if emailStr, ok := email.(string); ok {
			customClaims.Email = emailStr
		}
	}

	if isAdmin, exists := claims["is_admin"]; exists {
		if isAdminBool, ok := isAdmin.(bool); ok {
			customClaims.IsAdmin = isAdminBool
		}
	}

	if roles, exists := claims["roles"]; exists {
		if rolesSlice, ok := roles.([]interface{}); ok {
			customClaims.Roles = make([]string, len(rolesSlice))
			for i, role := range rolesSlice {
				if roleStr, ok := role.(string); ok {
					customClaims.Roles[i] = roleStr
				}
			}
		}
	}

	if subject, exists := claims["sub"]; exists {
		if subjectStr, ok := subject.(string); ok {
			customClaims.Subject = subjectStr
		}
	}

	return customClaims, nil
}
