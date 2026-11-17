package frontend

import (
	"embed"
	"shadmin/domain"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var Static embed.FS

func Register(r *gin.Engine) {
	fs, err := static.EmbedFolder(Static, "dist")
	if err != nil {
		panic("静态文件映射错误,请检查前端是否构建" + err.Error())
	}
	r.Use(static.Serve("/", fs))

	// 非后端路由交由 SPA 处理。
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") ||
			strings.HasPrefix(path, "/share/") ||
			strings.HasPrefix(path, "/swagger") ||
			strings.HasPrefix(path, "/client-access") {
			c.JSON(404, domain.RespError("API endpoint not found"))
			return
		}

		c.Header("Content-Type", "text/html")
		indexHTML, err := Static.ReadFile("dist/index.html")
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to load SPA"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", indexHTML)
	})
}
