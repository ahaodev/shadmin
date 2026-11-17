package contextutil

import (
	"strings"

	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// BearerToken 的 scheme 不区分大小写；令牌含空格或格式无效时返回 ok=false。
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

func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(constants.UserID); exists {
		if userIDStr, ok := userID.(string); ok {
			return userIDStr
		}
	}
	return ""
}
