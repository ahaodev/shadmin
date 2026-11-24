package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cacher "github.com/ahaodev/go-cache"
)

func newTestManager() *LoginSecurityManager {
	return NewLoginSecurityManager(
		cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute}),
	)
}

func TestLoginSecurityManager_LocksAfterMaxFailures(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()

	var attempts int
	for i := 0; i < m.MaxFailures(); i++ {
		attempts = m.RecordFailedAttempt(ctx, "alice")
	}

	if attempts != m.MaxFailures() {
		t.Fatalf("returned attempts = %d, want %d", attempts, m.MaxFailures())
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

func TestLoginSecurityManager_LockExpiresWithTTL(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()
	m.LockDuration = 20 * time.Millisecond

	for i := 0; i < m.MaxFailures(); i++ {
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

// brokenCacher 模拟缓存故障，验证登录计数失败时放行。
type brokenCacher struct{}

func (brokenCacher) SetIfAbsent(context.Context, string, string, string, time.Duration) (bool, error) {
	return false, errCacheDown
}

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

	for i := 0; i < m.MaxFailures()+1; i++ {
		m.RecordFailedAttempt(ctx, "alice")
	}

	if m.IsLocked(ctx, "alice") {
		t.Fatal("cache failure must not lock users out")
	}
}

// 并发失败不得丢计数，否则可绕过失败上限。
func TestLoginSecurityManager_ConcurrentFailuresAreCounted(t *testing.T) {
	ctx := context.Background()
	m := newTestManager()
	m.maxFailures = 1 << 30 // 只验证计数，不在中途触发锁定

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
