package domain

import (
	"context"
	"time"
)

type LoginLog struct {
	ID            string    `json:"id"`
	Email         string    `json:"email,omitempty"`
	LoginIP       string    `json:"login_ip"`
	UserAgent     string    `json:"user_agent"`
	Browser       string    `json:"browser,omitempty"`
	OS            string    `json:"os,omitempty"`
	Device        string    `json:"device,omitempty"`
	Status        string    `json:"status"`           // success, failed
	Source        string    `json:"source,omitempty"` // shadmin / provider 名
	FailureReason string    `json:"failure_reason,omitempty"`
	LoginTime     time.Time `json:"login_time"`
}

type CreateLoginLogRequest struct {
	Email         string `json:"email,omitempty"`
	LoginIP       string `json:"login_ip" binding:"required"`
	UserAgent     string `json:"user_agent" binding:"required"`
	Browser       string `json:"browser,omitempty"`
	OS            string `json:"os,omitempty"`
	Device        string `json:"device,omitempty"`
	Status        string `json:"status" binding:"required"` // success, failed
	Source        string `json:"source,omitempty"`          // shadmin / provider 名
	FailureReason string `json:"failure_reason,omitempty"`
}

type LoginLogPagedResult = PagedResult[*LoginLog]

type LoginLogQueryFilter struct {
	Email     string     `json:"email,omitempty"`
	LoginIP   string     `json:"login_ip,omitempty"`
	Status    string     `json:"status,omitempty"`
	Source    string     `json:"source,omitempty"`
	Browser   string     `json:"browser,omitempty"`
	OS        string     `json:"os,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	QueryParams
}

type LoginLogRepository interface {
	Create(ctx context.Context, loginLog *LoginLog) error
	Query(ctx context.Context, filter LoginLogQueryFilter) (*PagedResult[*LoginLog], error)
	ClearAll(ctx context.Context) error
}

type LoginLogUseCase interface {
	CreateLoginLog(ctx context.Context, request *CreateLoginLogRequest) (*LoginLog, error)
	ListLoginLogs(ctx context.Context, filter LoginLogQueryFilter) (*PagedResult[*LoginLog], error)
	ClearAllLoginLogs(ctx context.Context) error
}
