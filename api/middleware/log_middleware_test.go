package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// captureStdout 在 fn 执行期间捕获 os.Stdout 的输出（LogMiddleware 用 fmt.Print 直写 stdout）。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer file.Close()

	original := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = original }()

	fn()

	out, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(out)
}

func newLogEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(LogMiddleware())
	return engine
}

// TestLogMiddleware_LogsFullBodyWithoutTruncation 覆盖两点：请求体与响应体整段打印（无长度限制），
// 且 handler 收到的请求体与客户端发送的原始字节完全一致。
func TestLogMiddleware_LogsFullBodyWithoutTruncation(t *testing.T) {
	// 体量远大于历史上出现过的日志上限（1000 / 10000），用于证明现在不做任何截断
	blob := strings.Repeat("a", 30*1024)
	payload := `{"identifier":"admin","blob":"` + blob + `"}`

	var received []byte
	engine := newLogEngine()
	engine.POST("/auth/login", func(c *gin.Context) {
		received, _ = c.GetRawData()
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"blob": blob}})
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	log := captureStdout(t, func() {
		engine.ServeHTTP(httptest.NewRecorder(), req)
	})

	if string(received) != payload {
		t.Fatalf("handler 收到的请求体长度 = %d, 期望 %d 字节原样", len(received), len(payload))
	}
	if !strings.Contains(log, payload) {
		t.Fatalf("日志未包含完整请求体（日志长度 %d）", len(log))
	}
	if !strings.Contains(log, blob) {
		t.Fatalf("日志未包含完整响应体（日志长度 %d）", len(log))
	}
	if strings.Contains(log, "truncated") {
		t.Fatalf("日志不应出现截断标记:\n%s", log)
	}
}

// TestLogMiddleware_CapturesWriteString 覆盖 c.String 路径：gin 的 WriteString 不经过 Write。
func TestLogMiddleware_CapturesWriteString(t *testing.T) {
	engine := newLogEngine()
	engine.GET("/plain", func(c *gin.Context) {
		c.String(http.StatusOK, "plain body")
	})

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	recorder := httptest.NewRecorder()

	log := captureStdout(t, func() {
		engine.ServeHTTP(recorder, req)
	})

	if recorder.Body.String() != "plain body" {
		t.Fatalf("响应体 = %q, 期望 %q", recorder.Body.String(), "plain body")
	}
	if !strings.Contains(log, "plain body") {
		t.Fatalf("WriteString 响应未被记录:\n%s", log)
	}
}
