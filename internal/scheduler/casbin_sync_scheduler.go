package scheduler

import (
	"context"
	"log/slog"
	"shadmin/internal/casbin"
	"sync"
	"time"
)

type CasbinSyncScheduler struct {
	syncService *casbin.SyncService
	interval    time.Duration
	triggerChan chan struct{}
	running     bool
	stopChan    chan struct{}
	doneChan    chan struct{}
	mutex       sync.RWMutex
}

func NewCasbinSyncScheduler(syncService *casbin.SyncService, interval time.Duration) *CasbinSyncScheduler {
	if interval <= 0 {
		interval = time.Hour
	}
	return &CasbinSyncScheduler{
		syncService: syncService,
		interval:    interval,
		// 合并触发通知，避免阻塞提交 hook。
		triggerChan: make(chan struct{}, 1),
	}
}

// TriggerSync 合并提交后的同步请求；轮询兜底遗漏通知与远端变更。
func (s *CasbinSyncScheduler) TriggerSync() {
	select {
	case s.triggerChan <- struct{}{}:
	default:
	}
}

func (s *CasbinSyncScheduler) Start(ctx context.Context) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		slog.Warn("Casbin同步调度器已经在运行")
		return
	}

	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	s.stopChan = stopChan
	s.doneChan = doneChan
	s.running = true

	go s.run(ctx, s.interval, stopChan, doneChan)
}

// Stop 可重复或并发调用。
func (s *CasbinSyncScheduler) Stop() {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return
	}

	stopChan := s.stopChan
	doneChan := s.doneChan
	s.running = false
	s.mutex.Unlock()

	close(stopChan)
	<-doneChan // 等待本次运行的 goroutine 真正退出

	slog.Info("Casbin同步调度器已停止")
}

// run 使用 Start 传入的通道快照，避免读取可变字段。
func (s *CasbinSyncScheduler) run(ctx context.Context, interval time.Duration, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			slog.Info("收到停止信号，退出Casbin同步定时任务")
			return

		case <-ctx.Done():
			slog.Info("上下文已取消，退出Casbin同步定时任务")
			return

		case <-ticker.C:
			if err := s.performSync(ctx); err != nil {
				slog.Error("Casbin generation sync failed", "error", err)
			}

		case <-s.triggerChan:
			if err := s.performSync(ctx); err != nil {
				slog.Error("Casbin generation sync failed after trigger", "error", err)
			}
		}
	}
}

func (s *CasbinSyncScheduler) performSync(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.syncService.SyncIfChanged(syncCtx); err != nil {
		return err
	}

	return nil
}
