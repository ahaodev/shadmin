package auth

import (
	"context"
	"os"
	"shadmin/internal/cacher"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenConsumption(t *testing.T) {
	backends := map[string]cacher.Cacher{"memory": cacher.NewMemoryCache(cacher.MemoryConfig{})}
	if addr := os.Getenv("SHADMIN_TEST_REDIS"); addr != "" {
		c, err := cacher.NewForRuntime(cacher.RuntimeConfig{UseRedis: true, Redis: cacher.RedisConfig{Addr: addr}})
		if err != nil {
			t.Fatal(err)
		}
		backends["redis"] = c
	}
	for name, c := range backends {
		t.Run(name, func(t *testing.T) {
			defer c.Close(context.Background())
			// Two independently constructed adapters model two service instances.
			a, b := NewTokenBlacklist(c), NewTokenBlacklist(c)
			jti := t.Name() + time.Now().String()
			start := make(chan struct{})
			var wg sync.WaitGroup
			var winners atomic.Int32
			for i := range 32 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					adapter := a
					if i%2 == 0 {
						adapter = b
					}
					ok, err := adapter.Consume(t.Context(), jti, time.Now().Add(time.Minute))
					if err != nil {
						t.Error(err)
					}
					if ok {
						winners.Add(1)
					}
				}(i)
			}
			close(start)
			wg.Wait()
			if winners.Load() != 1 {
				t.Fatalf("winners = %d", winners.Load())
			}
			if ok, err := b.Consume(t.Context(), jti, time.Now().Add(time.Minute)); err != nil || ok {
				t.Fatalf("replay = %v, %v", ok, err)
			}
			if ok, err := a.Consume(t.Context(), "expired", time.Now().Add(-time.Second)); err != nil || ok {
				t.Fatalf("expired = %v, %v", ok, err)
			}
			if err := a.Add(t.Context(), "logout-"+jti, time.Now().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if ok, err := b.Consume(t.Context(), "logout-"+jti, time.Now().Add(time.Minute)); err != nil || ok {
				t.Fatalf("logout replay = %v, %v", ok, err)
			}
		})
	}
}
