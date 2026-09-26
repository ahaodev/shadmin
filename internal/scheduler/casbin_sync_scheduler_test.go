package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

// The scheduler tests exercise lifecycle behavior only; the interval is long enough that no tick runs.
const testInterval = time.Hour

func TestCasbinSyncSchedulerTriggerCoalesces(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)

	s.TriggerSync()
	s.TriggerSync()

	if got := len(s.triggerChan); got != 1 {
		t.Fatalf("queued trigger count = %d, want 1", got)
	}
}

func TestCasbinSyncSchedulerUsesHourlyFallbackByDefault(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, 0)
	if s.interval != time.Hour {
		t.Fatalf("default interval = %v, want %v", s.interval, time.Hour)
	}
}

func schedulerRunning(s *CasbinSyncScheduler) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

func TestCasbinSyncSchedulerStopIsIdempotent(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	s.Start(context.Background())

	s.Stop()
	s.Stop()

	if schedulerRunning(s) {
		t.Fatal("scheduler still running after Stop")
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

	if schedulerRunning(s) {
		t.Fatal("scheduler still running after concurrent Stop")
	}
}

func TestCasbinSyncSchedulerRestartAfterStop(t *testing.T) {
	s := NewCasbinSyncScheduler(nil, testInterval)
	ctx := context.Background()

	for round := range 3 {
		s.Start(ctx)
		if !schedulerRunning(s) {
			t.Fatalf("round %d: not running after Start", round)
		}

		s.Stop()
		if schedulerRunning(s) {
			t.Fatalf("round %d: still running after Stop", round)
		}
	}
}
