package userstatus

import (
	"context"
	"fmt"
	"time"

	"shadmin/internal/cachex"
)

// Store 抽象用户状态缓存后端。
// Get 仅查缓存，不回源自 DB；回源由 Cache 通过 Loader 完成。
// 内存/Redis 的后端选择由调用方通过注入的 cachex.Cacher 一次性决定。
type Store interface {
	// Get 返回缓存的状态；ok=false 表示未命中。
	Get(ctx context.Context, userID string) (status string, ok bool, err error)
	// Set 写入状态，ttl 为缓存有效期。
	Set(ctx context.Context, userID, status string, ttl time.Duration) error
	// Invalidate 删除一条缓存（允许重复删除）。
	Invalidate(ctx context.Context, userID string) error
}

const userStatusNS = "userstatus"

// NewStore 返回基于 cachex.Cacher 的 Store 实现。
func NewStore(cacher cachex.Cacher) Store {
	return &store{cacher: cacher}
}

type store struct {
	cacher cachex.Cacher
}

func (s *store) Get(ctx context.Context, userID string) (string, bool, error) {
	if userID == "" {
		return "", false, nil
	}
	v, ok, err := s.cacher.Get(ctx, userStatusNS, userID)
	if err != nil {
		return "", false, fmt.Errorf("get user status: %w", err)
	}
	if !ok {
		return "", false, nil
	}
	return v, true, nil
}

func (s *store) Set(ctx context.Context, userID, status string, ttl time.Duration) error {
	if userID == "" {
		return nil
	}
	return s.cacher.Set(ctx, userStatusNS, userID, status, ttl)
}

func (s *store) Invalidate(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	return s.cacher.Delete(ctx, userStatusNS, userID)
}
