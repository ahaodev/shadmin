package userstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store 抽象用户状态缓存后端，支持进程内存与 Redis 两种实现。
// Get 仅查缓存，不回源自 DB；回源由 Cache 通过 Loader 完成。
type Store interface {
	// Get 返回缓存的状态；ok=false 表示未命中。
	Get(ctx context.Context, userID string) (status string, ok bool, err error)
	// Set 写入状态，ttl 为缓存有效期。
	Set(ctx context.Context, userID, status string, ttl time.Duration) error
	// Invalidate 删除一条缓存（允许重复删除）。
	Invalidate(ctx context.Context, userID string) error
}

// NewMemoryStore 返回基于 go-cache 的进程内实现。
func NewMemoryStore(defaultTTL time.Duration) Store {
	return newMemoryStore(defaultTTL)
}

// NewRedisStore 返回基于 go-redis 的实现，依靠 key TTL 自动过期。
func NewRedisStore(client *redis.Client) Store {
	return &redisStore{client: client, keyPrefix: "userstatus:"}
}

type redisStore struct {
	client    *redis.Client
	keyPrefix string
}

func (s *redisStore) Get(ctx context.Context, userID string) (string, bool, error) {
	if userID == "" {
		return "", false, nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	v, err := s.client.Get(c, s.keyPrefix+userID).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get user status: %w", err)
	}
	return v, true, nil
}

func (s *redisStore) Set(ctx context.Context, userID, status string, ttl time.Duration) error {
	if userID == "" || ttl <= 0 {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.client.Set(c, s.keyPrefix+userID, status, ttl).Err()
}

func (s *redisStore) Invalidate(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = s.client.Del(c, s.keyPrefix+userID).Err()
	return nil
}
