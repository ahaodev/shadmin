package auth

import (
	"context"
	"strconv"
	"time"

	"shadmin/internal/cacher"
)

const loginFailNS = "auth:login:fail"

// LoginSecurityManager 按 identifier 统计连续登录失败次数，达到上限后短期拒绝登录。
type LoginSecurityManager struct {
	cacher cacher.Cacher

	MaxFailures  int           // 最大失败次数
	LockDuration time.Duration // 计数的有效期，即锁定窗口
}

// NewLoginSecurityManager 创建登录安全管理器，计数的过期由 cacher 负责。
func NewLoginSecurityManager(c cacher.Cacher) *LoginSecurityManager {
	if c == nil {
		panic("login security manager: cacher is required")
	}
	return &LoginSecurityManager{
		cacher:       c,
		MaxFailures:  3,           // 最大失败3次
		LockDuration: time.Minute, // 锁定1分钟
	}
}

// failCount 读取窗口内的失败次数
func (lsm *LoginSecurityManager) failCount(ctx context.Context, identifier string) int {
	raw, ok, err := lsm.cacher.Get(ctx, loginFailNS, identifier)
	if err != nil {
		log.Printf("login security: read fail count for %q failed: %v", identifier, err)
		return 0
	}
	if !ok {
		return 0
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("login security: discard corrupt fail count for %q: %v", identifier, err)
		return 0
	}
	return n
}

// IsLocked 检查该 identifier 是否已达失败上限。缓存不可用时返回 false（放行）。
func (lsm *LoginSecurityManager) IsLocked(ctx context.Context, identifier string) bool {
	return lsm.failCount(ctx, identifier) >= lsm.MaxFailures
}

// RecordFailedAttempt 记录一次失败并返回累计次数；返回值 >= MaxFailures 即已锁定。
func (lsm *LoginSecurityManager) RecordFailedAttempt(ctx context.Context, identifier string) int {
	n := lsm.failCount(ctx, identifier) + 1
	if err := lsm.cacher.Set(ctx, loginFailNS, identifier, strconv.Itoa(n), lsm.LockDuration); err != nil {
		log.Printf("login security: store fail count for %q failed: %v", identifier, err)
	}
	return n
}

// RecordSuccessfulLogin 清除失败计数。
func (lsm *LoginSecurityManager) RecordSuccessfulLogin(ctx context.Context, identifier string) {
	if err := lsm.cacher.Delete(ctx, loginFailNS, identifier); err != nil {
		log.Printf("login security: clear fail count for %q failed: %v", identifier, err)
	}
}
