package auth

import (
	"context"
	"time"

	"shadmin/domain"
	"shadmin/internal/cacher"
)

// NewTokenBlacklist 返回基于 cacher.Cacher 的黑名单实现（domain.TokenBlacklist）。
// 内存/Redis 的后端选择由调用方通过 cacher 一次性决定。
func NewTokenBlacklist(cacher cacher.Cacher) domain.TokenBlacklist {
	return &blacklist{cacher: cacher}
}

const blacklistNS = "jwt:blacklist"

type blacklist struct {
	cacher cacher.Cacher
}

func (b *blacklist) Add(ctx context.Context, jti string, expiry time.Time) error {
	if jti == "" {
		return nil
	}
	ttl := time.Until(expiry)
	if ttl <= 0 {
		return nil
	}
	return b.cacher.Set(ctx, blacklistNS, jti, "1", ttl)
}

func (b *blacklist) Exists(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	return b.cacher.Exists(ctx, blacklistNS, jti)
}
