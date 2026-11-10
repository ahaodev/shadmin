package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"shadmin/domain"
)

// Recovery logs panic details through Gin, returning only the public error contract.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, domain.RespError("internal server error"))
	})
}
