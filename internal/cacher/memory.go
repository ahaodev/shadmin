package cacher

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// MemoryConfig 配置内存版 Cacher。
type MemoryConfig struct {
	// CleanupInterval 过期项的主动回收周期，推荐 1m；传 0 则仅惰性回收。
	CleanupInterval time.Duration
}

// NewMemoryCache 返回基于 go-cache 的进程内实现。go-cache 本身线程安全，无需额外加锁。
func NewMemoryCache(cfg MemoryConfig, opts ...Option) Cacher {
	return &memoryCache{
		o: applyOptions(opts...),
		// 默认无过期：过期由每次 Set 的 expiration 参数决定。
		c: cache.New(cache.NoExpiration, cfg.CleanupInterval),
	}
}

type memoryCache struct {
	o *options
	c *cache.Cache
	// gadMu 仅用于 GetAndDelete，保证进程内"取+删"整体原子，
	// 防止并发 goroutine 同时取到同一 key（一次性语义所依赖）。
	gadMu sync.Mutex
}

func (m *memoryCache) tl(exp []time.Duration) time.Duration {
	if len(exp) > 0 {
		return exp[0]
	}
	return cache.NoExpiration
}

func (m *memoryCache) Set(_ context.Context, ns, key, value string, expiration ...time.Duration) error {
	m.c.Set(m.o.joinKey(ns, key), value, m.tl(expiration))
	return nil
}

func (m *memoryCache) Get(_ context.Context, ns, key string) (string, bool, error) {
	v, ok := m.c.Get(m.o.joinKey(ns, key))
	if !ok {
		return "", false, nil
	}
	s, _ := v.(string)
	return s, true, nil
}

func (m *memoryCache) Exists(_ context.Context, ns, key string) (bool, error) {
	_, ok := m.c.Get(m.o.joinKey(ns, key))
	return ok, nil
}

func (m *memoryCache) Delete(_ context.Context, ns, key string) error {
	m.c.Delete(m.o.joinKey(ns, key))
	return nil
}

func (m *memoryCache) GetAndDelete(_ context.Context, ns, key string) (string, bool, error) {
	full := m.o.joinKey(ns, key)
	// go-cache 无原生原子取删；用 gadMu 串行化"取+删"，
	// 确保并发 GetAndDelete 只有一个能取到值（一次性/限次语义依赖此原子性）。
	m.gadMu.Lock()
	defer m.gadMu.Unlock()
	v, ok := m.c.Get(full)
	if !ok {
		return "", false, nil
	}
	m.c.Delete(full)
	s, _ := v.(string)
	return s, true, nil
}

func (m *memoryCache) Iterator(ctx context.Context, ns string, fn func(ctx context.Context, key, value string) bool) error {
	// 直接遍历快照，避免长时间持锁。
	prefix := m.o.nsPrefix(ns)
	for k, item := range m.c.Items() {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		s, _ := item.Object.(string)
		if !fn(ctx, strings.TrimPrefix(k, prefix), s) {
			break
		}
	}
	return nil
}

func (m *memoryCache) Close(_ context.Context) error {
	m.c.Flush()
	return nil
}
