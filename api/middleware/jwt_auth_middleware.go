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

// JwtAuthMiddleware 校验 access token 并把 claims 注入 gin context。
// 若传入的 userStatusCache 非 nil，则在每个请求上检查用户状态：
// 状态非 active（或缓存报告为禁用）时返回 401，
// 让 admin 对账户的禁用/启用立刻生效，无需等到 access token 自然过期。
// 若传入的 tokenBlacklist 非 nil，则校验 jti 是否已登出。
func JwtAuthMiddleware(secret string, userStatusCache *auth.Cache, tokenBlacklist auth.JWTBlacklist) gin.HandlerFunc {
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

		// 黑名单校验：jti 已登出则拒绝；老 token 无 jti 视为不在黑名单。
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

		claims, err := tokenutil.ExtractAllClaimsFromToken(authToken, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
			c.Abort()
			return
		}

		// 状态检查：缓存未配置时退化为只校验 token 本身（保留向后兼容）。
		if userStatusCache != nil {
			status, err := userStatusCache.Get(c.Request.Context(), claims.ID)
			if err != nil || status != domain.UserStatusActive {
				c.JSON(http.StatusUnauthorized, domain.RespError("账户未启用或已停用"))
				c.Abort()
				return
			}
		}

		c.Set(constants.UserID, claims.ID)
		c.Set(constants.UserName, claims.Name)
		c.Set(constants.UserEmail, claims.Email)
		c.Set(constants.IsAdmin, claims.IsAdmin)
		c.Set(constants.UserRoles, claims.Roles)

		c.Next()
	}
}
