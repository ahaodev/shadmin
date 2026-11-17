package auth

import (
	"context"
	"log/slog"
	"shadmin/internal/cacher"
	"time"

	"shadmin/domain"
)

// DefaultTTL 限制失效通知遗漏时的用户状态缓存滞后时间。
const DefaultTTL = 30 * time.Second

const userStatusNS = "UserStatus"

type Loader interface {
	GetStatusByID(ctx context.Context, id string) (string, error)
}

type Cache struct {
	loader Loader
	cacher cacher.Cacher
	ttl    time.Duration
}

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
		slog.Info("user status cache get failed", "user_id", userID, "error", err)
	} else if ok {
		return status, nil
	}

	status, err := c.loader.GetStatusByID(ctx, userID)
	if err != nil {
		return "", err
	}

	if err := c.cacher.Set(ctx, userStatusNS, userID, status, c.ttl); err != nil {
		slog.Info("user status cache set failed", "user_id", userID, "error", err)
	}
	return status, nil
}

func (c *Cache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	if err := c.cacher.Delete(context.Background(), userStatusNS, userID); err != nil {
		slog.Info("user status cache delete failed", "user_id", userID, "error", err)
	}
}
