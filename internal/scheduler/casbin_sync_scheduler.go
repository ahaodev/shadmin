package scheduler

import (
	"context"
	"shadmin/internal/casbin"
	"shadmin/pkg"
	"sync"
	"time"
)

var log = pkg.Log

// CasbinSyncScheduler runs generation-triggered syncs with a periodic fallback.
type CasbinSyncScheduler struct {
	syncService *casbin.SyncService
	interval    time.Duration
	triggerChan chan struct{}
	running     bool
	stopChan    chan struct{}
	doneChan    chan struct{}
	mutex       sync.RWMutex
}

// NewCasbinSyncScheduler creates a Casbin sync scheduler.
func NewCasbinSyncScheduler(syncService *casbin.SyncService, interval time.Duration) *CasbinSyncScheduler {
	if interval <= 0 {
		interval = time.Hour
	}
	return &CasbinSyncScheduler{
		syncService: syncService,
		interval:    interval,
		// A buffered, coalescing trigger avoids blocking the Ent commit hook and
		// collapses bursts of authorization writes into one sync attempt.
		triggerChan: make(chan struct{}, 1),
	}
}

// TriggerSync requests an authorization snapshot refresh after a committed
// generation change. The request is best-effort and coalesced; the periodic
// poll remains the recovery path for missed triggers and other instances.
func (s *CasbinSyncScheduler) TriggerSync() {
	select {
	case s.triggerChan <- struct{}{}:
	default:
	}
}

// Start 启动定时同步任务
func (s *CasbinSyncScheduler) Start(ctx context.Context) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		log.Printf("WARN: Casbin同步调度器已经在运行")
		return
	}

	// stopChan/doneChan 每次启动重建，Stop 之后可以再次 Start。
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	s.stopChan = stopChan
	s.doneChan = doneChan
	s.running = true

	go s.run(ctx, s.interval, stopChan, doneChan)
}

// Stop 停止定时同步任务。可重复调用（含并发调用）：重复调用是安全的空操作。
func (s *CasbinSyncScheduler) Stop() {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return
	}

	stopChan := s.stopChan
	doneChan := s.doneChan
	// 先置 running=false 再释放锁：并发/重复的 Stop 会直接返回，不会重复 close。
	s.running = false
	s.mutex.Unlock()

	close(stopChan)
	<-doneChan // 等待本次运行的 goroutine 真正退出

	log.Printf(" Casbin同步调度器已停止")
}

// run 执行定时同步任务的主循环。interval 与通道由 Start 快照传入，不读取可变字段。
func (s *CasbinSyncScheduler) run(ctx context.Context, interval time.Duration, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Printf(" 收到停止信号，退出Casbin同步定时任务")
			return

		case <-ctx.Done():
			log.Printf(" 上下文已取消，退出Casbin同步定时任务")
			return

		case <-ticker.C:
			if err := s.performSync(ctx); err != nil {
				log.Printf("ERROR: Casbin generation sync failed: %v", err)
			}

		case <-s.triggerChan:
			if err := s.performSync(ctx); err != nil {
				log.Printf("ERROR: Casbin generation sync failed after trigger: %v", err)
			}
		}
	}
}

// performSync 执行一次同步操作
func (s *CasbinSyncScheduler) performSync(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.syncService.SyncIfChanged(syncCtx); err != nil {
		return err
	}

	return nil
}
