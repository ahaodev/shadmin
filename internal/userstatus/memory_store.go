package userstatus

import (
	"context"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// memoryStore 基于 patrickmn/go-cache，提供线程安全的进程内缓存。
type memoryStore struct {
	cache *cache.Cache
	mu    sync.RWMutex // 仅用于保护无锁访问路径；go-cache 本身已线程安全
}

func newMemoryStore(defaultTTL time.Duration) *memoryStore {
	if defaultTTL <= 0 {
		defaultTTL = DefaultTTL
	}
	return &memoryStore{
		cache: cache.New(defaultTTL, 2*defaultTTL),
	}
}

func (s *memoryStore) Get(_ context.Context, userID string) (string, bool, error) {
	if userID == "" {
		return "", false, nil
	}
	if v, ok := s.cache.Get(userID); ok {
		if status, ok := v.(string); ok {
			return status, true, nil
		}
	}
	return "", false, nil
}

func (s *memoryStore) Set(_ context.Context, userID, status string, _ time.Duration) error {
	if userID == "" {
		return nil
	}
	s.cache.SetDefault(userID, status)
	return nil
}

func (s *memoryStore) Invalidate(_ context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	s.cache.Delete(userID)
	return nil
}
