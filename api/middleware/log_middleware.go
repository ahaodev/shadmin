package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"shadmin/internal/constants"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogMiddleware 请求和响应日志中间件
func LogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()

		// 读取请求体
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 创建响应体捕获器
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		// 构建请求日志信息
		var logLines []string
		logLines = append(logLines, fmt.Sprintf("> 📥 REQUEST: %s %s", c.Request.Method, c.Request.URL.Path))
		logLines = append(logLines, "┌─────────────────────────────────────────────────────────────")

		if len(c.Request.URL.RawQuery) > 0 {
			logLines = append(logLines, fmt.Sprintf("│ 🔍 Query: %s", c.Request.URL.RawQuery))
		}

		// 只显示重要的请求头
		importantHeaders := []string{"Content-Type", constants.Authorization, "Accept"}
		for _, header := range importantHeaders {
			if value := c.Request.Header.Get(header); value != "" {
				// 对Authorization进行脱敏处理
				if header == constants.Authorization && len(value) > 20 {
					value = value[:20] + "..."
				}
				logLines = append(logLines, fmt.Sprintf("│ 📋 %s: %s", header, value))
			}
		}

		if len(requestBody) > 0 {
			logLines = append(logLines, fmt.Sprintf("│ 📦 Body: %s", string(requestBody)))
		}

		// 执行请求
		c.Next()

		// 计算请求耗时
		duration := time.Since(start)

		// 获取状态码颜色
		statusCode := c.Writer.Status()
		statusEmoji := getStatusEmoji(statusCode)

		// 添加响应信息到日志
		responseBody := w.body.String()
		logLines = append(logLines, fmt.Sprintf("│ %s RESPONSE: [%d] - %s", statusEmoji, statusCode, duration))

		if len(responseBody) > 0 {
			// 限制响应体长度以提高可读性
			if len(responseBody) > 5000 {
				logLines = append(logLines, fmt.Sprintf("│ 📤 %s... (truncated)", responseBody[:500]))
			} else {
				logLines = append(logLines, fmt.Sprintf("│ 📤 %s", responseBody))
			}
		}
		logLines = append(logLines, "└─────────────────────────────────────────────────────────────")

		// 一次性打印完整日志，避免中断
		log.Print(strings.Join(logLines, "\n"))
	}
}

// responseBodyWriter 用于捕获响应体
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// getStatusEmoji 根据状态码返回对应的表情符号
func getStatusEmoji(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "✅" // 成功
	case statusCode >= 300 && statusCode < 400:
		return "🔄" // 重定向
	case statusCode >= 400 && statusCode < 500:
		return "❌" // 客户端错误
	case statusCode >= 500:
		return "💥" // 服务器错误
	default:
		return "ℹ️" // 信息
	}
}
