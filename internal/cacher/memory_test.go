package cacher

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryCacheIncr(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(MemoryConfig{})

	if n, err := c.Incr(ctx, "ns", "k", time.Minute); err != nil || n != 1 {
		t.Fatalf("first Incr = %d, %v; want 1, nil", n, err)
	}
	if n, err := c.Incr(ctx, "ns", "k", time.Minute); err != nil || n != 2 {
		t.Fatalf("second Incr = %d, %v; want 2, nil", n, err)
	}
	if v, ok, err := c.Get(ctx, "ns", "k"); err != nil || !ok || v != "2" {
		t.Fatalf("Get = %q, %v, %v; want \"2\", true, nil", v, ok, err)
	}
}

func TestMemoryCacheIncrIsAtomic(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(MemoryConfig{})

	const writers, perWriter = 8, 100

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range perWriter {
				if _, err := c.Incr(ctx, "ns", "k", time.Minute); err != nil {
					t.Errorf("Incr: %v", err)
					return
				}
			}
		})
	}
	wg.Wait()

	v, ok, err := c.Get(ctx, "ns", "k")
	if err != nil || !ok {
		t.Fatalf("Get = %q, %v, %v; want a value", v, ok, err)
	}
	if want := "800"; v != want {
		t.Fatalf("counter = %q, want %q", v, want)
	}
}
