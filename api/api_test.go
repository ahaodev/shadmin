package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// ctx 取消后：等待在途请求完成再返回 nil，而不是直接断开连接。
func TestServe_GracefulShutdownWaitsForInflightRequests(t *testing.T) {
	addr := freeAddr(t)
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, addr, h) }()

	var body string
	reqDone := make(chan error, 1)
	go func() {
		for range 50 { // 等待监听就绪
			resp, err := http.Get("http://" + addr)
			if err != nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			reqDone <- nil
			return
		}
		reqDone <- context.DeadlineExceeded
	}()

	<-started
	cancel() // 请求处理中触发停机

	if err := <-done; err != nil {
		t.Fatalf("serve returned %v, want nil on graceful shutdown", err)
	}
	if err := <-reqDone; err != nil || body != "done" {
		t.Fatalf("in-flight request: body = %q, err = %v; want it to complete", body, err)
	}
}

// 端口被占用：把监听错误返回给调用方，而不是静默退出或 Fatal。
func TestServe_ReturnsListenError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	err = serve(context.Background(), l.Addr().String(), http.NewServeMux())
	if err == nil {
		t.Fatal("serve on an occupied port returned nil, want an error")
	}
}
