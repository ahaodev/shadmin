package domain

import (
	"context"
	"errors"
	"time"
)

const (
	DeviceAuthStatusPending    = "pending"
	DeviceAuthStatusAuthorized = "authorized"
	DeviceAuthStatusConsumed   = "consumed"
	DeviceAuthStatusExpired    = "expired"
	DeviceAuthStatusDenied     = "denied"
)

var (
	ErrDeviceAuthorizationPending = errors.New("authorization_pending")
	ErrDeviceSlowDown             = errors.New("slow_down")
	ErrDeviceExpired              = errors.New("expired_token")
	ErrDeviceInvalidCode          = errors.New("invalid_device_code")
	ErrDeviceConsumed             = errors.New("device_code_consumed")
	ErrDeviceAccessDenied         = errors.New("access_denied")
)

type DeviceAuthSession struct {
	ID              string
	DeviceCode      string
	UserCode        string
	ClientID        string
	ClientName      string
	Status          string
	UserID          string
	Interval        int
	InvalidAttempts int
	LastPolledAt    *time.Time
	ExpiresAt       time.Time
	AuthorizedAt    *time.Time
	ConsumedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DeviceCodeRequest struct {
	ClientID   string `json:"client_id" binding:"required"`
	ClientName string `json:"client_name,omitempty"`
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DeviceTokenRequest struct {
	ClientID   string `json:"client_id" binding:"required"`
	DeviceCode string `json:"device_code" binding:"required"`
}

type DeviceActivateRequest struct {
	UserCode string `json:"user_code" binding:"required"`
}

type DeviceActivateResponse struct {
	Status string `json:"status"`
}

type DeviceAuthRepository interface {
	Create(ctx context.Context, session *DeviceAuthSession) error
	GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthSession, error)
	GetByUserCode(ctx context.Context, userCode string) (*DeviceAuthSession, error)
	MarkAuthorized(ctx context.Context, userCode string, userID string, now time.Time) error
	ConsumeAuthorized(ctx context.Context, deviceCode string, now time.Time) (*DeviceAuthSession, error)
	UpdatePollState(ctx context.Context, deviceCode string, lastPolledAt time.Time, interval int) error
	IncrementInvalidAttempts(ctx context.Context, userCode string) (*DeviceAuthSession, error)
	Deny(ctx context.Context, userCode string) error
	Expire(ctx context.Context, deviceCode string, now time.Time) error
	DeleteExpired(ctx context.Context, now time.Time) error
}

type DeviceAuthUsecase interface {
	RequestCode(ctx context.Context, req DeviceCodeRequest, verificationURI string) (*DeviceCodeResponse, error)
	PollToken(ctx context.Context, req DeviceTokenRequest) (*LoginResponse, error)
	Activate(ctx context.Context, userID string, req DeviceActivateRequest) (*DeviceActivateResponse, error)
}
