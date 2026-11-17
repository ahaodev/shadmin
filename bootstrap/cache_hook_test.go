package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"shadmin/ent"
	"shadmin/internal/auth"
	"shadmin/internal/cacher"
)

func newTestEntClient(t *testing.T) *ent.Client {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client, err := ent.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	if err != nil {
		t.Fatalf("open ent client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return client
}

type countingLoader struct {
	calls  atomic.Int32
	status string
}

func (l *countingLoader) GetStatusByID(context.Context, string) (string, error) {
	l.calls.Add(1)
	return l.status, nil
}

func (l *countingLoader) callCount() int32 { return l.calls.Load() }

type cacheFixture struct {
	cache  *auth.Cache
	loader *countingLoader
	client *ent.Client
	ctx    context.Context
}

func newCacheFixture(t *testing.T) *cacheFixture {
	t.Helper()

	client := newTestEntClient(t)
	loader := &countingLoader{status: "active"}
	cache := auth.NewUserStatusCacher(loader, cacher.NewMemoryCache(cacher.MemoryConfig{}), time.Minute)

	app := &Application{DB: client, UserStatusCache: cache}
	app.registerUserStatusCacheHook()

	return &cacheFixture{cache: cache, loader: loader, client: client, ctx: context.Background()}
}

func (f *cacheFixture) createUser(t *testing.T, id string) {
	t.Helper()
	if err := f.client.User.Create().SetID(id).SetUsername(id).Exec(f.ctx); err != nil {
		t.Fatalf("create user %s: %v", id, err)
	}
}

func (f *cacheFixture) prime(t *testing.T, id string) {
	t.Helper()
	before := f.loader.callCount()
	if _, err := f.cache.Get(f.ctx, id); err != nil {
		t.Fatalf("prime cache for %s: %v", id, err)
	}
	if got := f.loader.callCount(); got != before+1 {
		t.Fatalf("loader calls = %d, want %d after priming %s", got, before+1, id)
	}
}

func TestUserStatusCacheHook_InvalidatesAfterCommit(t *testing.T) {
	f := newCacheFixture(t)
	f.createUser(t, "u1")
	f.prime(t, "u1")

	// 提交前不得失效，避免并发请求用旧值回填。
	tx, err := f.client.Tx(f.ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := tx.User.UpdateOneID("u1").SetStatus("inactive").Exec(f.ctx); err != nil {
		t.Fatalf("update in tx: %v", err)
	}
	if _, err := f.cache.Get(f.ctx, "u1"); err != nil {
		t.Fatalf("get during tx: %v", err)
	}
	if got := f.loader.callCount(); got != 1 {
		t.Fatalf("cache invalidated before commit: loader calls = %d, want 1", got)
	}

	f.loader.status = "inactive"
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	status, err := f.cache.Get(f.ctx, "u1")
	if err != nil {
		t.Fatalf("get after commit: %v", err)
	}
	if status != "inactive" {
		t.Fatalf("status after commit = %q, want %q (cache was not invalidated)", status, "inactive")
	}
	if got := f.loader.callCount(); got != 2 {
		t.Fatalf("loader calls after commit = %d, want 2", got)
	}
}

func TestUserStatusCacheHook_DoesNotInvalidateOnRollback(t *testing.T) {
	f := newCacheFixture(t)
	f.createUser(t, "u1")
	f.prime(t, "u1")

	tx, err := f.client.Tx(f.ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := tx.User.UpdateOneID("u1").SetStatus("inactive").Exec(f.ctx); err != nil {
		t.Fatalf("update in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if _, err := f.cache.Get(f.ctx, "u1"); err != nil {
		t.Fatalf("get after rollback: %v", err)
	}
	if got := f.loader.callCount(); got != 1 {
		t.Fatalf("cache invalidated on rollback: loader calls = %d, want 1", got)
	}
}

func TestUserStatusCacheHook_InvalidatesBulkTargets(t *testing.T) {
	f := newCacheFixture(t)
	f.createUser(t, "u1")
	f.createUser(t, "u2")
	f.prime(t, "u1")
	f.prime(t, "u2")
	if got := f.loader.callCount(); got != 2 {
		t.Fatalf("loader calls after priming = %d, want 2", got)
	}

	if err := f.client.User.Update().SetStatus("inactive").Exec(f.ctx); err != nil {
		t.Fatalf("bulk update: %v", err)
	}

	for _, id := range []string{"u1", "u2"} {
		if _, err := f.cache.Get(f.ctx, id); err != nil {
			t.Fatalf("get %s after bulk update: %v", id, err)
		}
	}
	if got := f.loader.callCount(); got != 4 {
		t.Fatalf("bulk update did not invalidate all matched users: loader calls = %d, want 4", got)
	}
}
