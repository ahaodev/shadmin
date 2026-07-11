package cacher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
	defaultRedisPoolSize     = 20
	defaultRedisMinIdleConns = 2
)

// RedisConfig 用于在无现成客户端时由本包自行构造 Redis 客户端。
type RedisConfig struct {
	Addr     string
	Username string
	Password string
	DB       int
}

func (cfg RedisConfig) options() *redis.Options {
	return &redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  defaultRedisDialTimeout,
		ReadTimeout:  defaultRedisReadTimeout,
		WriteTimeout: defaultRedisWriteTimeout,
		PoolSize:     defaultRedisPoolSize,
		MinIdleConns: defaultRedisMinIdleConns,
	}
}

// NewRedisClient 根据配置构造一个共享的 *redis.Client，并执行一次 Ping 校验。
func NewRedisClient(cfg RedisConfig) (*redis.Client, error) {
	cli := redis.NewClient(cfg.options())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return cli, nil
}

// NewClient 依据已校验的环境配置构造 Redis 客户端。
// 调用方负责在关闭时执行 Close()。
func NewClient(addr, password string, db int) (*redis.Client, error) {
	return NewRedisClient(RedisConfig{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// NewRedisCacheWithClient 复用已有 *redis.Client，适合共享连接池的场景；
// 客户端生命周期由调用方负责，Close 不会关闭它。
func NewRedisCacheWithClient(cli *redis.Client, opts ...Option) Cacher {
	return newRedisCache(cli, false, opts...)
}

// NewRedisCacheWithClusterClient 复用已有集群客户端；Close 不会关闭它。
func NewRedisCacheWithClusterClient(cli *redis.ClusterClient, opts ...Option) Cacher {
	return newRedisCache(cli, false, opts...)
}

// redisClienter 抽象出需要的命令子集，使单机与集群客户端均可注入。
type redisClienter interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Close() error
}

func newRedisCache(cli redisClienter, ownClient bool, opts ...Option) Cacher {
	return &redisCache{o: applyOptions(opts...), cli: cli, ownClient: ownClient}
}

type redisCache struct {
	o         *options
	cli       redisClienter
	ownClient bool // true 表示客户端由本实例创建，Close 时需关闭
}

func (r *redisCache) tl(exp []time.Duration) time.Duration {
	if len(exp) > 0 {
		return exp[0]
	}
	return 0 // Redis Set 传 0 表示无 TTL，等价于永不过期
}

func (r *redisCache) Set(ctx context.Context, ns, key, value string, expiration ...time.Duration) error {
	return r.cli.Set(ctx, r.o.joinKey(ns, key), value, r.tl(expiration)).Err()
}

func (r *redisCache) Get(ctx context.Context, ns, key string) (string, bool, error) {
	v, err := r.cli.Get(ctx, r.o.joinKey(ns, key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

// GetAndDelete 使用 GETDEL（Redis 6.2+）原子取删，满足 nonce/黑名单的一次性语义。
func (r *redisCache) GetAndDelete(ctx context.Context, ns, key string) (string, bool, error) {
	v, err := r.cli.GetDel(ctx, r.o.joinKey(ns, key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (r *redisCache) Exists(ctx context.Context, ns, key string) (bool, error) {
	n, err := r.cli.Exists(ctx, r.o.joinKey(ns, key)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Delete 直接 DEL，幂等无需先 Exists，省一次 RTT。
func (r *redisCache) Delete(ctx context.Context, ns, key string) error {
	return r.cli.Del(ctx, r.o.joinKey(ns, key)).Err()
}

func (r *redisCache) Iterator(ctx context.Context, ns string, fn func(ctx context.Context, key, value string) bool) error {
	pattern := r.o.joinKey(ns, "*")
	prefix := r.o.nsPrefix(ns)
	const scanCount = 100
	var cursor uint64
	// SCAN 无锁遍历，期间键可能被并发修改；SCAN 可能重复返回同一键，由调用方按需幂等处理。
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		keys, next, err := r.cli.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return err
		}
		for _, k := range keys {
			v, err := r.cli.Get(ctx, k).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				return err
			}
			if !fn(ctx, strings.TrimPrefix(k, prefix), v) {
				return nil
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

func (r *redisCache) Close(_ context.Context) error {
	if !r.ownClient {
		// 客户端由调用方持有，不在此关闭，避免与共享方的关闭逻辑重复。
		return nil
	}
	return r.cli.Close()
}
