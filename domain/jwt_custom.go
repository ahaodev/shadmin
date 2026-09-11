package domain

import (
	"github.com/golang-jwt/jwt/v5"
)

type JwtCustomClaims struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	IsAdmin bool     `json:"is_admin"`
	Roles   []string `json:"roles"`
	jwt.RegisteredClaims
}

// JTI 返回登出黑名单使用的 jti。必须提供：外层 ID 字段遮蔽了内嵌的 RegisteredClaims.ID，
// 直接写 claims.ID 会拿到用户 ID。
func (c *JwtCustomClaims) JTI() string { return c.RegisteredClaims.ID }

type JwtCustomRefreshClaims struct {
	ID string `json:"id"`
	jwt.RegisteredClaims
}

// JTI 返回登出黑名单使用的 jti，原因同 JwtCustomClaims.JTI。
func (c *JwtCustomRefreshClaims) JTI() string { return c.RegisteredClaims.ID }
