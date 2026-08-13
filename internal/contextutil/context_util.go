package contextutil

import (
	"net"
	"strings"

	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// GetUserID 从gin context中获取用户ID
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(constants.UserID); exists {
		if userIDStr, ok := userID.(string); ok {
			return userIDStr
		}
	}
	return ""
}

// GetClientIP 获取客户端真实 IP 地址，依次信任 X-Forwarded-For、X-Real-IP，最后回退到 RemoteAddr。
func GetClientIP(c *gin.Context) string {
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); net.ParseIP(ip) != nil {
			return ip
		}
	}

	if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}
