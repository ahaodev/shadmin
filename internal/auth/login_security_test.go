package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"shadmin/internal/cacher"
)

// newTestManager 用内存版 cacher 构造管理器。计数过期由 cacher 的 TTL 负责。
func newTestManager() *LoginSecurityManager {
	return NewLoginSecurityManager(
		cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute}),
	)
}

func TestLoginSecurityManager_LocksAfterMaxFailures(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()

	var attempts int
	for i := 0; i < m.MaxFailures; i++ {
		attempts = m.RecordFailedAttempt(ctx, "alice")
	}

	if attempts != m.MaxFailures {
		t.Fatalf("returned attempts = %d, want %d", attempts, m.MaxFailures)
	}
	if !m.IsLocked(ctx, "alice") {
		t.Fatal("alice should be locked after reaching MaxFailures")
	}
}

func TestLoginSecurityManager_SuccessfulLoginClearsRecord(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()

	m.RecordFailedAttempt(ctx, "alice")
	m.RecordSuccessfulLogin(ctx, "alice")

	if m.IsLocked(ctx, "alice") {
		t.Fatal("alice should not be locked after successful login")
	}
	if attempts := m.RecordFailedAttempt(ctx, "alice"); attempts != 1 {
		t.Fatalf("attempts = %d, want 1 after counter was cleared", attempts)
	}
}

// 计数靠 TTL 归零：窗口内不再失败，锁定自动解除，且新的一轮从 1 重新计数，
// 不会把上一轮的失败累加进来。
func TestLoginSecurityManager_LockExpiresWithTTL(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()
	m.LockDuration = 20 * time.Millisecond

	for i := 0; i < m.MaxFailures; i++ {
		m.RecordFailedAttempt(ctx, "alice")
	}
	if !m.IsLocked(ctx, "alice") {
		t.Fatal("alice should be locked")
	}

	time.Sleep(3 * m.LockDuration)

	if m.IsLocked(ctx, "alice") {
		t.Fatal("alice should be unlocked once the counter TTL expires")
	}
	if attempts := m.RecordFailedAttempt(ctx, "alice"); attempts != 1 {
		t.Fatalf("attempts = %d, want 1 after window expired", attempts)
	}
}

var errCacheDown = errors.New("cache unavailable")

// brokenCacher 模拟缓存不可用，用于固定 fail-open 行为：
// 缓存故障时登录必须放行，不能把所有用户挡在门外。
type brokenCacher struct{}

func (brokenCacher) Set(context.Context, string, string, string, ...time.Duration) error {
	return errCacheDown
}
func (brokenCacher) Get(context.Context, string, string) (string, bool, error) {
	return "", false, errCacheDown
}
func (brokenCacher) GetAndDelete(context.Context, string, string) (string, bool, error) {
	return "", false, errCacheDown
}
func (brokenCacher) Incr(context.Context, string, string, ...time.Duration) (int64, error) {
	return 0, errCacheDown
}
func (brokenCacher) Exists(context.Context, string, string) (bool, error) {
	return false, errCacheDown
}
func (brokenCacher) Delete(context.Context, string, string) error { return errCacheDown }
func (brokenCacher) Iterator(context.Context, string, func(context.Context, string, string) bool) error {
	return errCacheDown
}
func (brokenCacher) Close(context.Context) error { return nil }

func TestLoginSecurityManager_CacheFailureFailsOpen(t *testing.T) {
	ctx := context.Background()
	m := NewLoginSecurityManager(brokenCacher{})

	for i := 0; i < m.MaxFailures+1; i++ {
		m.RecordFailedAttempt(ctx, "alice")
	}

	if m.IsLocked(ctx, "alice") {
		t.Fatal("cache failure must not lock users out")
	}
}

// 并发失败必须逐一计入：读改写会让多个 goroutine 读到同一个旧值互相覆盖，
// 使攻击者用并发请求绕过失败上限。
func TestLoginSecurityManager_ConcurrentFailuresAreCounted(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()
	m.MaxFailures = 1 << 30 // 只验证计数，不在中途触发锁定

	const writers, perWriter = 8, 25

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range perWriter {
				m.RecordFailedAttempt(ctx, "alice")
			}
		})
	}
	wg.Wait()

	if got := m.failCount(ctx, "alice"); got != writers*perWriter {
		t.Fatalf("fail count = %d, want %d", got, writers*perWriter)
	}
}
