package middleware

import (
	"errors"
	"net/http"
	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// UserStateMiddleware 校验用户账户状态。
func UserStateMiddleware(userStatusCache *auth.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(constants.UserID)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, domain.RespError("Not authorized"))
			c.Abort()
			return
		}

		if userStatusCache != nil {
			status, err := userStatusCache.Get(c.Request.Context(), userID)
			if err != nil {
				if errors.Is(err, domain.ErrUserDisabled) {
					c.JSON(http.StatusUnauthorized, domain.RespError("账户未启用或已停用"))
				} else {
					c.JSON(http.StatusServiceUnavailable, domain.RespError("用户状态暂时无法验证"))
				}
				c.Abort()
				return
			}
			if status != domain.UserStatusActive {
				c.JSON(http.StatusUnauthorized, domain.RespError("账户未启用或已停用"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
