package auth

import (
	"context"
	"shadmin/internal/cacher"
	"shadmin/pkg"
	"time"

	"shadmin/domain"
)

var log = pkg.Log

// DefaultTTL 是默认缓存有效期。短 TTL 保证即使 ent hook 失效通知遗漏，
// 被禁用用户的 token 也会在 DefaultTTL 内失效。
const DefaultTTL = 30 * time.Second

const userStatusNS = "UserStatus"

// Loader fetches the user's status from the source of truth.
type Loader interface {
	GetStatusByID(ctx context.Context, id string) (string, error)
}

// Cache 是用户状态缓存，结合 Loader（DB 回源）与 Cacher（缓存后端）。
type Cache struct {
	loader Loader
	cacher cacher.Cacher
	ttl    time.Duration
}

// NewUserStatusCacher 返回一个 Cache。cacher 决定缓存落地，ttl 控制写入有效期。
func NewUserStatusCacher(loader Loader, cacher cacher.Cacher, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if loader == nil {
		panic("loader is required")
	}
	if cacher == nil {
		panic("cacher is required")
	}
	return &Cache{loader: loader, cacher: cacher, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", domain.ErrUserDisabled
	}

	if status, ok, err := c.cacher.Get(ctx, userStatusNS, userID); err != nil {
		log.Printf("cache get failed for user %s: %v", userID, err)
	} else if ok {
		return status, nil
	}

	status, err := c.loader.GetStatusByID(ctx, userID)
	if err != nil {
		// 缓存错误不阻塞流程：视为禁用并让下次请求重试。
		return "", domain.ErrUserDisabled
	}

	if err := c.cacher.Set(ctx, userStatusNS, userID, status, c.ttl); err != nil {
		log.Printf("cache set failed for user %s: %v", userID, err)
	}
	return status, nil
}

// Invalidate drops the cached entry for userID.
func (c *Cache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	if err := c.cacher.Delete(context.Background(), userStatusNS, userID); err != nil {
		log.Printf("cache delete failed for user %s: %v", userID, err)
	}
}
