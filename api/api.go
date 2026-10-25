package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"shadmin/api/route"
	"shadmin/bootstrap"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 15 * time.Second
)

// SetupRoutes 注册所有路由；失败意味着服务不可用，必须由调用方终止启动。
func SetupRoutes(app *bootstrap.Application) error {
	if err := route.Setup(app, app.ApiEngine); err != nil {
		return fmt.Errorf("setup routes: %w", err)
	}
	return nil
}

// Run 启动 HTTP 服务并阻塞，直到收到 SIGINT/SIGTERM 后优雅停机。
// 正常停机返回 nil，调用方随后可安全释放数据库、缓存等资源。
func Run(app *bootstrap.Application) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return serve(ctx, app.Env.Port, app.ApiEngine)
}

// serve 监听 addr 直到 ctx 取消，随后在 shutdownTimeout 内等待在途请求完成。
func serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		return err // 启动失败（如端口被占用）
	case <-ctx.Done():
	}

	slog.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}
