// Package tokenblacklist 提供 JWT 登出黑名单。
// 仅在用户主动登出时写入；过期后由底层 cacher.Cacher 自动清理。
// 缓存后端（进程内存或 Redis）由注入的 cacher.Cacher 决定，本包不再自行实现双后端。
package tokenblacklist

import (
	"context"
	"time"

	"shadmin/internal/cacher"
)

// Blacklist 记录已登出的 JWT jti，直到其原始过期时间。
type Blacklist interface {
	// Add 将 jti 加入黑名单直到 expiry；expiry 已过则直接忽略。
	Add(ctx context.Context, jti string, expiry time.Time) error
	// Exists 检查 jti 是否在黑名单中且仍有效。
	Exists(ctx context.Context, jti string) (bool, error)
	// Close 释放底层资源（仅对自建客户端的 Redis Cacher 有实际动作）。
	Close() error
}

// New 返回基于 cacher.Cacher 的黑名单实现。
// 内存/Redis 的后端选择由调用方通过 cacher 一次性决定。
func New(cacher cacher.Cacher) Blacklist {
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

func (b *blacklist) Close() error {
	return b.cacher.Close(context.Background())
}
