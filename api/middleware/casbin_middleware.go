package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"shadmin/domain"
	"shadmin/internal/casbin"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// CasbinMiddleware API权限鉴权中间件
type CasbinMiddleware struct {
	Checker domain.PermissionChecker
}

// NewCasbinMiddleware 创建新的 Casbin 中间件实例
func NewCasbinMiddleware(checker domain.PermissionChecker) *CasbinMiddleware {
	return &CasbinMiddleware{
		Checker: checker,
	}
}

// CheckAPIPermission API网关权限检查中间件
func (m *CasbinMiddleware) CheckAPIPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(constants.UserID)

		path := c.Request.URL.Path
		method := c.Request.Method

		hasPermission, err := m.Checker.CheckPermission(c.Request.Context(), userID, path, method)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, casbin.ErrSnapshotStale) {
				status = http.StatusServiceUnavailable
			} else {
				slog.Error("API 权限检查失败", "user_id", userID, "method", method, "path", path, "error", err)
			}
			c.JSON(status, domain.RespError("权限检查失败"))
			c.Abort()
			return
		}

		if !hasPermission {
			slog.Warn("API 权限不足", "user_id", userID, "method", method, "path", path)
			c.JSON(http.StatusForbidden, domain.RespError("权限不足"))
			c.Abort()
			return
		}

		c.Next()
	}
}
