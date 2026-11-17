//go:build !production

package route

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func setupSwagger(gin *gin.Engine) {
	gin.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
