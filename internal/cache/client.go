// Package cache 提供 Redis 客户端工厂与轻量封装。
// 仅在 bootstrap.Env.RedisEnabled() 时实例化；否则后端各自走内存默认。
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient 依据已校验的环境配置构造 Redis 客户端。
// 调用方负责在关闭时执行 Close()。
func NewClient(addr, password string, db int) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return cli, nil
}
