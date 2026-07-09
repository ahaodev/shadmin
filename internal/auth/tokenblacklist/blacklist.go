// Package tokenblacklist 提供 JWT 登出黑名单的内存与 Redis 两种实现。
// 仅在用户主动登出时写入；过期后由底层存储自动清理。
package tokenblacklist

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Blacklist 记录已登出的 JWT jti，直到其原始过期时间。
type Blacklist interface {
	// Add 将 jti 加入黑名单直到 expiry；expiry 已过则直接忽略。
	Add(ctx context.Context, jti string, expiry time.Time) error
	// Exists 检查 jti 是否在黑名单中且仍有效。
	Exists(ctx context.Context, jti string) (bool, error)
	// Close 释放底层资源（内存实现的清理 goroutine）。
	Close() error
}

// NewMemoryBlacklist 返回进程内 map 实现，后台 goroutine 清理过期项。
func NewMemoryBlacklist() Blacklist {
	b := &memoryBlacklist{
		items:  make(map[string]time.Time),
		stopCh: make(chan struct{}),
	}
	go b.startCleanup()
	return b
}

// NewRedisBlacklist 返回基于 go-redis 的实现，依赖 key TTL 自动过期。
func NewRedisBlacklist(client *redis.Client) Blacklist {
	return &redisBlacklist{client: client, keyPrefix: "jwt:blacklist:"}
}

type memoryBlacklist struct {
	mu       sync.Mutex
	items    map[string]time.Time
	stopCh   chan struct{}
	stopOnce sync.Once
}

func (b *memoryBlacklist) Add(_ context.Context, jti string, expiry time.Time) error {
	if jti == "" || time.Now().After(expiry) {
		return nil
	}
	b.mu.Lock()
	b.items[jti] = expiry
	b.mu.Unlock()
	return nil
}

func (b *memoryBlacklist) Exists(_ context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	exp, ok := b.items[jti]
	if !ok {
		return false, nil
	}
	if now.After(exp) {
		delete(b.items, jti)
		return false, nil
	}
	return true, nil
}

func (b *memoryBlacklist) Close() error {
	b.stopOnce.Do(func() { close(b.stopCh) })
	return nil
}

func (b *memoryBlacklist) startCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			b.mu.Lock()
			for jti, exp := range b.items {
				if now.After(exp) {
					delete(b.items, jti)
				}
			}
			b.mu.Unlock()
		case <-b.stopCh:
			return
		}
	}
}

type redisBlacklist struct {
	client    *redis.Client
	keyPrefix string
}

func (b *redisBlacklist) Add(ctx context.Context, jti string, expiry time.Time) error {
	if jti == "" {
		return nil
	}
	ttl := time.Until(expiry)
	if ttl <= 0 {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return b.client.Set(c, b.keyPrefix+jti, "1", ttl).Err()
}

func (b *redisBlacklist) Exists(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	n, err := b.client.Exists(c, b.keyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (b *redisBlacklist) Close() error { return nil }
