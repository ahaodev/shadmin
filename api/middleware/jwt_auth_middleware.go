package middleware

import (
	"net/http"
	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/constants"
	"shadmin/internal/tokenutil"
	"strings"

	"github.com/gin-gonic/gin"
)

// JwtAuthMiddleware 校验 access token 的合法性、黑名单状态，并把 claims 注入 gin context。
func JwtAuthMiddleware(secret string, tokenBlacklist auth.JWTBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		t := strings.Split(authHeader, " ")
		if len(t) != 2 {
			c.JSON(http.StatusUnauthorized, domain.RespError("Not authorized"))
			c.Abort()
			return
		}
		authToken := t[1]
		authorized, err := tokenutil.IsAuthorized(authToken, secret)
		if !authorized {
			c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
			c.Abort()
			return
		}

		claims, err := tokenutil.ExtractAllClaimsFromToken(authToken, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
			c.Abort()
			return
		}

		if tokenBlacklist != nil {
			if jti, jErr := tokenutil.ExtractJTI(authToken, secret); jErr == nil && jti != "" {
				revoked, rErr := tokenBlacklist.Exists(c.Request.Context(), jti)
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
