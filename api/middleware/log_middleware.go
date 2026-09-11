package middleware

import (
	"bytes"
	"fmt"
	"io"
	"shadmin/internal/constants"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// maxLogBodyLen 日志中请求/响应体的最大展示长度，超出则截断
const maxLogBodyLen = 1000

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

		// 执行请求
		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()
		statusEmoji := getStatusEmoji(statusCode)
		responseBody := w.body.String()

		// 组装日志：标题行（Result 合并进首行）+ 缩进详情，无边框
		var lines []string

		target := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		result := fmt.Sprintf("[%d] %s %s", statusCode, statusEmoji, duration)

		// ▶ 图标按状态码着色：2xx 绿、4xx/5xx 红，其余无色
		icon := "▶"
		switch {
		case statusCode >= 200 && statusCode < 300:
			icon = "\x1b[32m▶\x1b[0m"
		case statusCode >= 400:
			icon = "\x1b[31m▶\x1b[0m"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s %s", icon, c.Request.Method, target, result))

		// 只显示重要的请求头（Authorization 脱敏）
		importantHeaders := []string{"Content-Type", constants.Authorization, "Accept"}
		for _, header := range importantHeaders {
			if value := c.Request.Header.Get(header); value != "" {
				if header == constants.Authorization && len(value) > 20 {
					value = value[:20] + "..."
				}
				lines = append(lines, fmt.Sprintf("    Header : %s: %s", header, value))
			}
		}

		// 请求内容（请求体），仅在存在时输出，置于 Resp 之前
		if len(requestBody) > 0 {
			lines = append(lines, fmt.Sprintf("    Request : %s", truncateLog(requestBody)))
		}

		if len(responseBody) > 0 {
			lines = append(lines, fmt.Sprintf("    Resp   : %s", truncateLog([]byte(responseBody))))
		}

		// 末尾空一行，便于在连续请求之间区分
		lines = append(lines, "")
		fmt.Print(strings.Join(lines, "\n"))
	}
}

// truncateLog 截断超长内容，并标注原始长度
func truncateLog(data []byte) string {
	if len(data) <= maxLogBodyLen {
		return string(data)
	}
	return fmt.Sprintf("%s... (truncated, %d chars)", string(data[:maxLogBodyLen]), len(data))
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
