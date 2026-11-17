package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"shadmin/internal/constants"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogMiddleware 的请求体和响应体不脱敏、不截断，仅供开发调试。
func LogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		var requestBody []byte
		if c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				// 读取失败也回填已读部分，供后续 handler 使用。
				slog.Warn("LogMiddleware: failed to read request body", "error", err)
			}
			requestBody = body
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()
		statusEmoji := getStatusEmoji(statusCode)
		responseBody := w.body.String()

		var lines []string

		target := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		result := fmt.Sprintf("[%d] %s %s", statusCode, statusEmoji, duration)

		icon := "▶"
		switch {
		case statusCode >= 200 && statusCode < 300:
			icon = "\x1b[32m▶\x1b[0m"
		case statusCode >= 400:
			icon = "\x1b[31m▶\x1b[0m"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s %s", icon, c.Request.Method, target, result))

		importantHeaders := []string{"Content-Type", constants.Authorization, "Accept"}
		for _, header := range importantHeaders {
			if value := c.Request.Header.Get(header); value != "" {
				if header == constants.Authorization && len(value) > 20 {
					value = value[:20] + "..."
				}
				lines = append(lines, fmt.Sprintf("    Header : %s: %s", header, value))
			}
		}

		if len(requestBody) > 0 {
			lines = append(lines, fmt.Sprintf("    Request : %s", requestBody))
		}

		if len(responseBody) > 0 {
			lines = append(lines, fmt.Sprintf("    Resp   : %s", responseBody))
		}

		lines = append(lines, "")
		fmt.Print(strings.Join(lines, "\n"))
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// WriteString 需单独捕获，因为 Gin 的该路径不经过 Write。
func (r *responseBodyWriter) WriteString(s string) (int, error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

func getStatusEmoji(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "✅"
	case statusCode >= 300 && statusCode < 400:
		return "🔄"
	case statusCode >= 400 && statusCode < 500:
		return "❌"
	case statusCode >= 500:
		return "💥"
	default:
		return "ℹ️"
	}
}
