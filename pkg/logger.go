package pkg

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const logFile = ".logs/app.log"

// Logger wraps slog.Logger with printf-style helpers.
type Logger struct{ *slog.Logger }

var Log = newLoggerWith(io.MultiWriter(os.Stdout, &lumberjack.Logger{
	Filename: logFile,
	MaxSize:  100, // MB
	MaxAge:   7,   // days
	Compress: true,
}))

func newLoggerWith(w io.Writer) Logger {
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Value = slog.StringValue(a.Value.Time().Format(TimeFormat))
			case slog.SourceKey:
				if src, ok := a.Value.Any().(*slog.Source); ok {
					a.Value = slog.StringValue(fmt.Sprintf("%s:%d", filepath.Base(src.File), src.Line))
				}
			}
			return a
		},
	})
	return Logger{slog.New(handler)}
}

// With returns a Logger that includes the given key-value attributes.
func (l Logger) With(args ...any) Logger { return Logger{l.Logger.With(args...)} }

func (l Logger) Debugf(format string, args ...any) { l.logf(slog.LevelDebug, format, args...) }
func (l Logger) Infof(format string, args ...any)  { l.logf(slog.LevelInfo, format, args...) }
func (l Logger) Warnf(format string, args ...any)  { l.logf(slog.LevelWarn, format, args...) }
func (l Logger) Errorf(format string, args ...any) { l.logf(slog.LevelError, format, args...) }

// Fatalf logs at error level and exits the process.
func (l Logger) Fatalf(format string, args ...any) {
	l.logf(slog.LevelError, format, args...)
	os.Exit(1)
}

// logf keeps the caller's source location instead of this wrapper's.
func (l Logger) logf(level slog.Level, format string, args ...any) {
	ctx := context.Background()
	if !l.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip Callers, logf, and the exported helper
	r := slog.NewRecord(time.Now(), level, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(ctx, r)
}
