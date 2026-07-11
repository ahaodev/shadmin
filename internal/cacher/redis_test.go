package cacher

import (
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
)

func TestRedisConfigOptions(t *testing.T) {
	cfg := RedisConfig{Addr: "127.0.0.1:6379", Username: "", Password: "", DB: 0}
	opts := cfg.options()

	if opts.Addr != cfg.Addr {
		t.Fatalf("expected addr %q, got %q", cfg.Addr, opts.Addr)
	}
	if opts.Username != cfg.Username {
		t.Fatalf("expected username %q, got %q", cfg.Username, opts.Username)
	}
	if opts.Password != cfg.Password {
		t.Fatalf("expected password %q, got %q", cfg.Password, opts.Password)
	}
	if opts.DB != cfg.DB {
		t.Fatalf("expected db %d, got %d", cfg.DB, opts.DB)
	}
	if opts.DialTimeout != defaultRedisDialTimeout {
		t.Fatalf("expected dial timeout %v, got %v", defaultRedisDialTimeout, opts.DialTimeout)
	}
	if opts.ReadTimeout != defaultRedisReadTimeout {
		t.Fatalf("expected read timeout %v, got %v", defaultRedisReadTimeout, opts.ReadTimeout)
	}
	if opts.WriteTimeout != defaultRedisWriteTimeout {
		t.Fatalf("expected write timeout %v, got %v", defaultRedisWriteTimeout, opts.WriteTimeout)
	}
	if opts.PoolSize != defaultRedisPoolSize {
		t.Fatalf("expected pool size %d, got %d", defaultRedisPoolSize, opts.PoolSize)
	}
	if opts.MinIdleConns != defaultRedisMinIdleConns {
		t.Fatalf("expected min idle conns %d, got %d", defaultRedisMinIdleConns, opts.MinIdleConns)
	}
}

func TestApplyOptions(t *testing.T) {
	o := applyOptions(WithDelimiter("|"))
	if o.Delimiter != "|" {
		t.Fatalf("expected delimiter %q, got %q", "|", o.Delimiter)
	}
	if o.joinKey("ns", "key") != "ns|key" {
		t.Fatalf("expected joined key %q, got %q", "ns|key", o.joinKey("ns", "key"))
	}
	if o.nsPrefix("ns") != "ns|" {
		t.Fatalf("expected prefix %q, got %q", "ns|", o.nsPrefix("ns"))
	}
}

func TestMemoryCacheTTL(t *testing.T) {
	m := &memoryCache{o: applyOptions(), c: nil}
	if got := m.tl(nil); got != cache.NoExpiration {
		t.Fatalf("expected no expiration default, got %v", got)
	}
	if got := m.tl([]time.Duration{time.Second}); got != time.Second {
		t.Fatalf("expected explicit ttl %v, got %v", time.Second, got)
	}
}
