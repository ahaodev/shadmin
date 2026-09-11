package middleware

import (
	"net/http"
	"strings"

	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/constants"
	"shadmin/internal/tokenservice"

	"github.com/gin-gonic/gin"
)

// JwtAuthMiddleware 校验 access token 的合法性、黑名单状态，并把 claims 注入 gin context。
func JwtAuthMiddleware(secret string, tokenBlacklist auth.JWTBlacklist) gin.HandlerFunc {
	tokenService := tokenservice.NewTokenService()

	return func(c *gin.Context) {
		authToken, ok := bearerToken(c.Request.Header.Get("Authorization"))
		if !ok {
			c.JSON(http.StatusUnauthorized, domain.RespError("Not authorized"))
			c.Abort()
			return
		}

		claims, err := tokenService.ParseAccessClaims(authToken, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
			c.Abort()
			return
		}

		if tokenBlacklist != nil {
			// 黑名单以 jti 为键：无 jti 的令牌无法吊销，直接拒绝
			if claims.JTI() == "" {
				c.JSON(http.StatusUnauthorized, domain.RespError("令牌无效"))
				c.Abort()
				return
			}

			revoked, rErr := tokenBlacklist.Exists(c.Request.Context(), claims.JTI())
			if rErr != nil {
				c.JSON(http.StatusUnauthorized, domain.RespError("令牌无法验证"))
				c.Abort()
				return
			}
			if revoked {
				c.JSON(http.StatusUnauthorized, domain.RespError("令牌已登出"))
				c.Abort()
				return
			}
		}

		c.Set(constants.UserID, claims.ID)
		c.Set(constants.UserName, claims.Name)
		c.Set(constants.UserEmail, claims.Email)
		c.Set(constants.IsAdmin, claims.IsAdmin)
		c.Set(constants.UserRoles, claims.Roles)
		c.Set(constants.UserSubject, claims.Subject)

		c.Next()
	}
}

// bearerToken 从 Authorization 头中取出 Bearer 令牌。
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || token == "" || strings.Contains(token, " ") {
		return "", false
	}
	if !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}
