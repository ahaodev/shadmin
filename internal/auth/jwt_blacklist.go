package auth

import (
	"context"
	"time"

	"shadmin/domain"

	cacher "github.com/ahaodev/go-cache"
)

func NewTokenBlacklist(cacher cacher.Cacher) domain.TokenBlacklist {
	return &blacklist{cacher: cacher}
}

const blacklistNS = "jwt:blacklist"

type blacklist struct {
	cacher cacher.Cacher
}

func (b *blacklist) Consume(ctx context.Context, jti string, expiry time.Time) (bool, error) {
	ttl := time.Until(expiry)
	if jti == "" || ttl <= 0 {
		return false, nil
	}
	return b.cacher.SetIfAbsent(ctx, blacklistNS, jti, "1", ttl)
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
