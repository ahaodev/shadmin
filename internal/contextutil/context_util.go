package contextutil

import (
	"strings"

	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// BearerToken 从 Authorization 头中取出 Bearer 令牌。
// scheme 大小写不敏感，令牌内不允许出现空格；缺失或格式不合法时 ok=false。
func BearerToken(header string) (token string, ok bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || token == "" || strings.Contains(token, " ") {
		return "", false
	}
	if !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}

// GetUserID 从gin context中获取用户ID
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(constants.UserID); exists {
		if userIDStr, ok := userID.(string); ok {
			return userIDStr
		}
	}
	return ""
}
