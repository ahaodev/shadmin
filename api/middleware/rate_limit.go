package middleware

import (
	"net/http"
	"sync"
	"time"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type ipRateLimitEntry struct {
	count       int
	windowStart time.Time
}

type ipRateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	hits        map[string]*ipRateLimitEntry
	lastCleanup time.Time
}

// RateLimitByIP limits requests from each client IP for the configured window.
func RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &ipRateLimiter{
		limit:       limit,
		window:      window,
		hits:        make(map[string]*ipRateLimitEntry),
		lastCleanup: time.Now(),
	}

	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, domain.RespError("too_many_requests"))
			return
		}
		c.Next()
	}
}

func (l *ipRateLimiter) allow(key string) bool {
	if key == "" {
		key = "unknown"
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.window {
		for ip, entry := range l.hits {
			if now.Sub(entry.windowStart) >= l.window {
				delete(l.hits, ip)
			}
		}
		l.lastCleanup = now
	}

	entry, ok := l.hits[key]
	if !ok || now.Sub(entry.windowStart) >= l.window {
		l.hits[key] = &ipRateLimitEntry{count: 1, windowStart: now}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}
