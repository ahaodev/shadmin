package clierr

import (
	"errors"
	"fmt"
	"os"
)

// CLI 退出码
const (
	ExitOK          = 0
	ExitGeneric     = 1
	ExitUsage       = 2
	ExitNetwork     = 3
	ExitUnauth      = 4
	ExitForbidden   = 5
	ExitNotFound    = 6
	ExitServerError = 7
)

// CLIError 承载 CLI 退出码 + 消息
type CLIError struct {
	Code    int
	Message string
	Details map[string]any
}

func (e *CLIError) Error() string {
	return e.Message
}

func New(code int, msg string) *CLIError {
	return &CLIError{Code: code, Message: msg}
}

func Wrap(code int, err error, prefix string) *CLIError {
	if err == nil {
		return nil
	}
	return &CLIError{Code: code, Message: fmt.Sprintf("%s: %s", prefix, err.Error())}
}

// ExitCode 返回一个 error 对应的退出码
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ExitGeneric
}

// Fatal 打印错误到 stderr 并以对应退出码退出
func Fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error: "+err.Error())
	os.Exit(ExitCode(err))
}
