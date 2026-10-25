package pkg

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

const logFile = ".logs/app.log"

var logLevel slog.LevelVar

var LogWriter = io.MultiWriter(os.Stdout, &lumberjack.Logger{
	Filename:   logFile,
	MaxSize:    5, // MB
	MaxBackups: 14,
	MaxAge:     7, // days
	Compress:   true,
	LocalTime:  true,
})

func InitLogger() {
	slog.SetDefault(slog.New(slog.NewTextHandler(LogWriter, &slog.HandlerOptions{
		AddSource: true,
		Level:     &logLevel,
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
	})))
}

// SetLogLevel 调整全局日志级别。
func SetLogLevel(l slog.Level) { logLevel.Set(l) }
