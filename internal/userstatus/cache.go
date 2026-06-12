// Package userstatus fronts UserRepository.GetStatusByID with a TTL cache
// so the JWT middleware and login/refresh flows can check whether a user
// is still active without hitting the DB on every request.
//
// The cache is best-effort: a stale entry is acceptable because the worst
// case is a single request seeing the old status, after which the next
// refresh re-reads from the DB. Invalidation is done by TTL and by an
// explicit Invalidate(userID) call from a User ent hook when the user's
// status changes. The cache does not need to be persistent across
// restarts.
//
// Implementation note: this package is a thin wrapper around
// github.com/patrickmn/go-cache, which already provides thread-safe
// in-memory storage, per-item expiration, and a background janitor.
package userstatus

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"

	"shadmin/domain"
)

// DefaultTTL is the default cache entry lifetime. Kept short so that a
// disabled user's token revokes within at most DefaultTTL even if the
// ent hook miss-fires.
const DefaultTTL = 30 * time.Second

// Loader fetches the user's status from the source of truth.
type Loader interface {
	GetStatusByID(ctx context.Context, id string) (string, error)
}

// Cache is a thread-safe TTL cache for user statuses, built on top of
// go-cache. A zero value is unusable; construct one with New.
type Cache struct {
	loader Loader
	cache  *cache.Cache
}

// New returns a Cache that loads from loader with the given ttl. If ttl
// is non-positive, DefaultTTL is used. The cleanup interval is set to
// double the TTL so expired entries are reaped without thrashing.
func New(loader Loader, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		loader: loader,
		cache:  cache.New(ttl, 2*ttl),
	}
}

// Get returns the user's status. On a cache miss it falls back to the
// loader, caches the result, and returns it. If the user does not exist
// in the source of truth, returns domain.ErrUserDisabled (we treat
// missing users the same as disabled users on the auth path).
func (c *Cache) Get(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", domain.ErrUserDisabled
	}

	if v, ok := c.cache.Get(userID); ok {
		if status, ok := v.(string); ok {
			return status, nil
		}
	}

	status, err := c.loader.GetStatusByID(ctx, userID)
	if err != nil {
		// Treat missing or unreadable users as disabled on the auth path.
		// We don't cache the error itself; next request retries.
		return "", domain.ErrUserDisabled
	}

	c.cache.SetDefault(userID, status)
	return status, nil
}

// Invalidate drops the cached entry for userID. The next Get will
// reload from the source of truth.
func (c *Cache) Invalidate(userID string) {
	if userID == "" {
		return
	}
	c.cache.Delete(userID)
}

// InvalidateAll clears the cache. Used in tests and on shutdown.
func (c *Cache) InvalidateAll() {
	c.cache.Flush()
}
