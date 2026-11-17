package auth

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"shadmin/internal/cacher"
)

const loginFailNS = "auth:login:fail"

type LoginSecurityManager struct {
	cacher cacher.Cacher

	maxFailures  int
	LockDuration time.Duration // 计数的有效期，即锁定窗口
}

func NewLoginSecurityManager(c cacher.Cacher) *LoginSecurityManager {
	if c == nil {
		panic("login security manager: cacher is required")
	}
	return &LoginSecurityManager{
		cacher:       c,
		maxFailures:  3,
		LockDuration: time.Minute,
	}
}

func (lsm *LoginSecurityManager) MaxFailures() int { return lsm.maxFailures }

func (lsm *LoginSecurityManager) failCount(ctx context.Context, identifier string) int {
	raw, ok, err := lsm.cacher.Get(ctx, loginFailNS, identifier)
	if err != nil {
		slog.Info("login security: read fail count failed", "identifier", identifier, "error", err)
		return 0
	}
	if !ok {
		return 0
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		slog.Info("login security: discard corrupt fail count", "identifier", identifier, "error", err)
		return 0
	}
	return n
}

// IsLocked 在缓存不可用时返回 false（放行）。
func (lsm *LoginSecurityManager) IsLocked(ctx context.Context, identifier string) bool {
	return lsm.failCount(ctx, identifier) >= lsm.maxFailures
}

func (lsm *LoginSecurityManager) RecordFailedAttempt(ctx context.Context, identifier string) int {
	n, err := lsm.cacher.Incr(ctx, loginFailNS, identifier, lsm.LockDuration)
	if err != nil {
		slog.Info("login security: record fail count failed", "identifier", identifier, "error", err)
		return 0
	}
	return int(n)
}

func (lsm *LoginSecurityManager) RecordSuccessfulLogin(ctx context.Context, identifier string) {
	if err := lsm.cacher.Delete(ctx, loginFailNS, identifier); err != nil {
		slog.Info("login security: clear fail count failed", "identifier", identifier, "error", err)
	}
}
