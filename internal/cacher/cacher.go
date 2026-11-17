package cacher

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cacher 按命名空间隔离键。
type Cacher interface {
	// SetIfAbsent 原子写入；键未过期时不覆盖。
	SetIfAbsent(ctx context.Context, ns, key, value string, expiration time.Duration) (bool, error)
	// Set 使用首个 expiration，未传则永不过期。
	Set(ctx context.Context, ns, key, value string, expiration ...time.Duration) error
	// Incr 原子自增，缺失时从 1 开始；expiration 同 Set，每次自增刷新 TTL。
	Incr(ctx context.Context, ns, key string, expiration ...time.Duration) (int64, error)
	// Get 未命中时返回 ok=false。
	Get(ctx context.Context, ns, key string) (string, bool, error)
	// GetAndDelete 原子取删，未命中时返回 ok=false。
	GetAndDelete(ctx context.Context, ns, key string) (string, bool, error)
	Exists(ctx context.Context, ns, key string) (bool, error)
	// Delete 允许重复删除。
	Delete(ctx context.Context, ns, key string) error
	// Iterator 的回调返回 false 时终止遍历。
	Iterator(ctx context.Context, ns string, fn func(ctx context.Context, key, value string) bool) error
	Close(ctx context.Context) error
}

type RuntimeConfig struct {
	UseRedis bool
	Redis    RedisConfig
	Memory   MemoryConfig
	Client   *redis.Client
}

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

type Option func(*options)

// WithDelimiter 的默认分隔符为 ":"。
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

func (o *options) joinKey(ns, key string) string {
	return fmt.Sprintf("%s%s%s", ns, o.Delimiter, key)
}

func (o *options) nsPrefix(ns string) string {
	return fmt.Sprintf("%s%s", ns, o.Delimiter)
}
