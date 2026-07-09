// Package userstatus fronts UserRepository.GetStatusByID with a TTL cache
// so the JWT middleware and login/refresh flows can check whether a user
// is still active without hitting the DB on every request.
//
// 后端由 Store 决定：进程内存（newMemoryStore，go-cache）或 Redis（newRedisStore，
// 依靠 key TTL）。Cache 自身只负责回源与协调。
package userstatus

import (
	"context"
	"time"

	"shadmin/domain"
)

// DefaultTTL 是默认缓存有效期。短 TTL 保证即使 ent hook 失效通知遗漏，
// 被禁用用户的 token 也会在 DefaultTTL 内失效。
const DefaultTTL = 30 * time.Second

// Loader fetches the user's status from the source of truth.
type Loader interface {
	GetStatusByID(ctx context.Context, id string) (string, error)
}

// Cache 是用户状态缓存，结合 Loader（DB 回源）与 Store（缓存后端）。
type Cache struct {
	loader Loader
	store  Store
	ttl    time.Duration
}

// New 返回一个 Cache。store 决定缓存落地，ttl 控制 Redis 写入 TTL 与内存默认 TTL。
// 传入 store 为 nil 时使用内存 store（便于调用方传 nil 退化）。
func New(loader Loader, store Store, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if store == nil {
		store = newMemoryStore(ttl)
	}
	return &Cache{loader: loader, store: store, ttl: ttl}
}

// Get returns the user's status. On a cache miss it falls back to the
// loader, caches the result, and returns it. If the user does not exist
// in the source of truth, returns domain.ErrUserDisabled.
func (c *Cache) Get(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", domain.ErrUserDisabled
	}

	if status, ok, _ := c.store.Get(ctx, userID); ok {
		return status, nil
	}

	status, err := c.loader.GetStatusByID(ctx, userID)
	if err != nil {
		// 缓存错误不阻塞流程：视为禁用并让下次请求重试。
		return "", domain.ErrUserDisabled
	}

	_ = c.store.Set(ctx, userID, status, c.ttl)
	return status, nil
}

// Invalidate drops the cached entry for userID.
func (c *Cache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	_ = c.store.Invalidate(context.Background(), userID)
}

// InvalidateAll is retained for tests; only meaningful for memory store.
func (c *Cache) InvalidateAll() {
	// 仅内存 store 支持批量清理；Redis 模式依赖 TTL，无需显式刷洗。
	if ms, ok := c.store.(*memoryStore); ok {
		ms.cache.Flush()
	}
}
