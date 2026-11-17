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

// SetupRoutes 失败时调用方必须终止启动。
func SetupRoutes(app *bootstrap.Application) error {
	if err := route.Setup(app, app.ApiEngine); err != nil {
		return fmt.Errorf("setup routes: %w", err)
	}
	return nil
}

// Run 阻塞至 SIGINT/SIGTERM，然后优雅停机。
func Run(app *bootstrap.Application) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return serve(ctx, app.Env.Port, app.ApiEngine)
}

// serve 在 ctx 取消后等待在途请求，最长 shutdownTimeout。
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
		return err
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
