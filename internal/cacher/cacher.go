package cacher

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cacher 抽象带命名空间的通用 key-value 缓存。
type Cacher interface {
	// Set 写入（或覆盖）一对键值；expiration 取首个非空值，未传则永不过期。
	Set(ctx context.Context, ns, key, value string, expiration ...time.Duration) error
	// Get 读取键值；ok=false 表示未命中。
	Get(ctx context.Context, ns, key string) (string, bool, error)
	// GetAndDelete 原子地读取并删除键值；ok=false 表示未命中。
	GetAndDelete(ctx context.Context, ns, key string) (string, bool, error)
	// Exists 检查键是否存在。
	Exists(ctx context.Context, ns, key string) (bool, error)
	// Delete 删除键（允许重复删除）。
	Delete(ctx context.Context, ns, key string) error
	// Iterator 遍历 ns 命名空间下全部键值；回调返回 false 立即终止遍历。
	Iterator(ctx context.Context, ns string, fn func(ctx context.Context, key, value string) bool) error
	// Close 释放底层资源。
	Close(ctx context.Context) error
}

// RuntimeConfig 用于运行时初始化统一缓存实现。
// 仅会根据 UseRedis 选择并初始化其中一个实现：Redis 或 Memory。
type RuntimeConfig struct {
	UseRedis bool
	Redis    RedisConfig
	Memory   MemoryConfig
	Client   *redis.Client
}

// NewForRuntime 根据运行时配置创建统一缓存实现。
// 业务层只需传入配置即可获取 Cacher 实例，而无需关心底层是 Redis 还是内存实现。
func NewForRuntime(cfg RuntimeConfig) (Cacher, error) {
	if cfg.UseRedis {
		if cfg.Client != nil {
			return NewRedisCacheWithClient(cfg.Client), nil
		}
		cli, err := NewRedisClient(cfg.Redis)
		if err != nil {
			return nil, err
		}
		return NewRedisCacheWithClient(cli), nil
	}
	return NewMemoryCache(cfg.Memory), nil
}

const defaultDelimiter = ":"

type options struct {
	Delimiter string
}

// Option 配置 Cacher 构造行为。
type Option func(*options)

// WithDelimiter 设置命名空间与键之间的分隔符，默认 ":"。
func WithDelimiter(d string) Option {
	return func(o *options) { o.Delimiter = d }
}

func applyOptions(opts ...Option) *options {
	o := &options{Delimiter: defaultDelimiter}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// joinKey 将 ns 与 key 以分隔符拼成实际存储键。
func (o *options) joinKey(ns, key string) string {
	return fmt.Sprintf("%s%s%s", ns, o.Delimiter, key)
}

// nsPrefix 返回 Iterator 使用的匹配前缀，例如 "ns:"。
func (o *options) nsPrefix(ns string) string {
	return fmt.Sprintf("%s%s", ns, o.Delimiter)
}
