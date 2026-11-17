package usecase

import (
	"context"
	"log/slog"
	"shadmin/domain"
	"strings"
	"time"
)

// writeLoginLog 异步落库；脱离请求取消，避免请求结束后丢失日志。
func writeLoginLog(parent context.Context, logUsecase domain.LoginLogUseCase, meta domain.LoginMeta, source, status, failureReason, email string) {
	if logUsecase == nil {
		return
	}
	logRequest := &domain.CreateLoginLogRequest{
		Email:         email,
		LoginIP:       meta.ClientIP,
		UserAgent:     meta.UserAgent,
		Status:        status,
		Source:        source,
		FailureReason: failureReason,
	}
	ctx := context.WithoutCancel(parent)
	go func() {
		if _, err := logUsecase.CreateLoginLog(ctx, logRequest); err != nil {
			slog.Warn("failed to record login log", "error", err)
		}
	}()
}

type loginLogUsecase struct {
	loginLogRepository domain.LoginLogRepository
	contextTimeout     time.Duration
}

func NewLoginLogUsecase(loginLogRepository domain.LoginLogRepository, timeout time.Duration) domain.LoginLogUseCase {
	return &loginLogUsecase{
		loginLogRepository: loginLogRepository,
		contextTimeout:     timeout,
	}
}

func (luc *loginLogUsecase) parseUserAgent(userAgent string) (browser, os, device string) {
	ua := strings.ToLower(userAgent)

	switch {
	case strings.Contains(ua, "firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
		browser = "Chrome"
	case strings.Contains(ua, "edg"):
		browser = "Edge"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		browser = "Safari"
	case strings.Contains(ua, "opera"):
		browser = "Opera"
	case strings.Contains(ua, "ie") || strings.Contains(ua, "trident"):
		browser = "Internet Explorer"
	default:
		browser = "Unknown"
	}

	switch {
	case strings.Contains(ua, "windows nt 10.0"):
		os = "Windows 10"
	case strings.Contains(ua, "windows nt 6.3"):
		os = "Windows 8.1"
	case strings.Contains(ua, "windows nt 6.1"):
		os = "Windows 7"
	case strings.Contains(ua, "windows"):
		os = "Windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os x"):
		os = "macOS"
	case strings.Contains(ua, "linux"):
		os = "Linux"
	case strings.Contains(ua, "android"):
		os = "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		os = "iOS"
	default:
		os = "Unknown"
	}

	switch {
	case strings.Contains(ua, "mobile"):
		device = "Mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		device = "Tablet"
	default:
		device = "Desktop"
	}

	return browser, os, device
}

func (luc *loginLogUsecase) CreateLoginLog(ctx context.Context, request *domain.CreateLoginLogRequest) (*domain.LoginLog, error) {
	ctx, cancel := context.WithTimeout(ctx, luc.contextTimeout)
	defer cancel()

	browser, os, device := luc.parseUserAgent(request.UserAgent)

	if request.Browser == "" {
		request.Browser = browser
	}
	if request.OS == "" {
		request.OS = os
	}
	if request.Device == "" {
		request.Device = device
	}

	if request.Source == "" {
		request.Source = domain.UserSourceLocal
	}

	loginLog := &domain.LoginLog{
		Email:         request.Email,
		LoginIP:       request.LoginIP,
		UserAgent:     request.UserAgent,
		Browser:       request.Browser,
		OS:            request.OS,
		Device:        request.Device,
		Status:        request.Status,
		Source:        request.Source,
		FailureReason: request.FailureReason,
		LoginTime:     time.Now(),
	}

	if err := luc.loginLogRepository.Create(ctx, loginLog); err != nil {
		return nil, err
	}

	return loginLog, nil
}

func (luc *loginLogUsecase) ListLoginLogs(ctx context.Context, filter domain.LoginLogQueryFilter) (*domain.PagedResult[*domain.LoginLog], error) {
	ctx, cancel := context.WithTimeout(ctx, luc.contextTimeout)
	defer cancel()
	return luc.loginLogRepository.Query(ctx, filter)
}

func (luc *loginLogUsecase) ClearAllLoginLogs(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, luc.contextTimeout)
	defer cancel()
	return luc.loginLogRepository.ClearAll(ctx)
}
