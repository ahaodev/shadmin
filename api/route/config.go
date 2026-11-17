package route

import (
	"fmt"
	"net/http"
	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type ServerConfig struct {
	TrustedProxies     []string
	MaxMultipartMemory int64
	RedirectSlash      bool
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		TrustedProxies:     []string{"127.0.0.1"},
		MaxMultipartMemory: 1024 << 20, //1G
		RedirectSlash:      true,
	}
}

func (c *ServerConfig) Apply(engine *gin.Engine) error {
	if err := engine.SetTrustedProxies(c.TrustedProxies); err != nil {
		return fmt.Errorf("failed to set trusted proxies: %w", err)
	}

	engine.MaxMultipartMemory = c.MaxMultipartMemory
	engine.RedirectTrailingSlash = c.RedirectSlash
	engine.HandleMethodNotAllowed = true
	engine.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(http.StatusNotFound, domain.RespError("API endpoint not found"))
	})
	engine.NoMethod(func(ctx *gin.Context) {
		ctx.JSON(http.StatusMethodNotAllowed, domain.RespError("Method not allowed"))
	})

	return nil
}
