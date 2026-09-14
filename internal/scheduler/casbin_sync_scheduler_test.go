package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

// 调度器只依赖 syncService 做实际同步；生命周期测试用 nil 即可（interval 取 1h，测试期间不会触发 tick）。
const testInterval = time.Hour

func TestCasbinSyncSchedulerStopIsIdempotent(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	s.Start(context.Background())

	s.Stop()
	s.Stop() // 修复前：第二次 close(stopChan) 会 panic

	if s.IsRunning() {
		t.Fatal("IsRunning = true after Stop")
	}
}

func TestCasbinSyncSchedulerStopConcurrent(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	s.Start(context.Background())

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
		}()
	}
	wg.Wait()

	if s.IsRunning() {
		t.Fatal("IsRunning = true after concurrent Stop")
	}
}

func TestCasbinSyncSchedulerRestartAfterStop(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	ctx := context.Background()

	// 修复前：stopChan 只创建一次，Stop 之后的 Start 会立刻收到已关闭的 channel 而退出。
	for round := range 3 {
		s.Start(ctx)
		if !s.IsRunning() {
			t.Fatalf("round %d: not running after Start", round)
		}

		s.Stop()
		if s.IsRunning() {
			t.Fatalf("round %d: still running after Stop", round)
		}
	}
}

func TestCasbinSyncSchedulerSetIntervalWhileRunning(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	s.Start(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.SetInterval(2 * testInterval)
	}()

	s.Stop()
	wg.Wait()

	if got := s.GetStatus().Interval; got != 2*testInterval {
		t.Fatalf("Interval = %v, want %v", got, 2*testInterval)
	}
}
