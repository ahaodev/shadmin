package cacher

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type MemoryConfig struct {
	// CleanupInterval 为 0 时仅惰性回收。
	CleanupInterval time.Duration
}

func NewMemoryCache(cfg MemoryConfig, opts ...Option) Cacher {
	return &memoryCache{
		o: applyOptions(opts...),
		c: cache.New(cache.NoExpiration, cfg.CleanupInterval),
	}
}

type memoryCache struct {
	o *options
	c *cache.Cache
	// gadMu 保证取删原子性。
	gadMu sync.Mutex
	// incrMu 保证自增原子性。
	incrMu sync.Mutex
}

func (m *memoryCache) tl(exp []time.Duration) time.Duration {
	if len(exp) > 0 {
		return exp[0]
	}
	return cache.NoExpiration
}

func (m *memoryCache) SetIfAbsent(ctx context.Context, ns, key, value string, expiration time.Duration) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return m.c.Add(m.o.joinKey(ns, key), value, expiration) == nil, nil
}

func (m *memoryCache) Set(_ context.Context, ns, key, value string, expiration ...time.Duration) error {
	m.c.Set(m.o.joinKey(ns, key), value, m.tl(expiration))
	return nil
}

func (m *memoryCache) Incr(_ context.Context, ns, key string, expiration ...time.Duration) (int64, error) {
	full := m.o.joinKey(ns, key)
	m.incrMu.Lock()
	defer m.incrMu.Unlock()

	var n int64
	if v, ok := m.c.Get(full); ok {
		s, _ := v.(string)
		n, _ = strconv.ParseInt(s, 10, 64) // 损坏值按 0 处理，重新从 1 计数
	}
	n++
	m.c.Set(full, strconv.FormatInt(n, 10), m.tl(expiration))
	return n, nil
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
