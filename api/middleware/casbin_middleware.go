package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shadmin/domain"
	"shadmin/internal/casbin"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// CasbinMiddleware API权限鉴权中间件
type CasbinMiddleware struct {
	CasManager casbin.Manager
}

// NewCasbinMiddleware 创建新的 Casbin 中间件实例
func NewCasbinMiddleware(CasManager casbin.Manager) *CasbinMiddleware {
	return &CasbinMiddleware{
		CasManager: CasManager,
	}
}

// CheckAPIPermission API网关权限检查中间件
func (m *CasbinMiddleware) CheckAPIPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(constants.UserID)

		path := c.Request.URL.Path
		method := c.Request.Method

		if m.shouldSkipPermissionCheck(path) {
			c.Next()
			return
		}

		hasPermission, err := m.CasManager.CheckPermission(userID, path, method)
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

// shouldSkipPermissionCheck 判断是否跳过权限检查
func (m *CasbinMiddleware) shouldSkipPermissionCheck(path string) bool {
	skipPaths := constants.GetAPIPathsToSkipPermissionCheck()

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}
