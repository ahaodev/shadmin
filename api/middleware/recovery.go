package middleware

import (
	"net/http"
	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, domain.RespError("internal server error"))
	})
}
