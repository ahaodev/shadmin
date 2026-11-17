package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"shadmin/domain"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, domain.RespError("internal server error"))
	})
}
