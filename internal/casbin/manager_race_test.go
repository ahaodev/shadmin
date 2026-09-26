package casbin

import (
	"sync"
	"testing"

	"github.com/casbin/casbin/v3"
)

// Concurrent requests must keep evaluating complete snapshots while the sync worker publishes replacements.
func TestManagerConcurrentSnapshotPublication(t *testing.T) {
	manager, first := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, "u1", "r1")
		mustAddPolicy(t, enforcer, "r1", "/api/x", "GET")
	})
	second, err := newEnforcer()
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}
	mustAddRoleForUser(t, second, "u1", "r1")
	mustAddPolicy(t, second, "r1", "/api/x", "GET")

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range iterations {
			if allowed, err := manager.CheckPermission("u1", "/api/x", "GET"); err != nil || !allowed {
				t.Errorf("CheckPermission = (%v, %v), want (true, nil)", allowed, err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for generation := int64(1); generation <= iterations; generation++ {
			enforcer := first
			if generation%2 == 0 {
				enforcer = second
			}
			if err := manager.ReplaceSnapshot(enforcer, generation); err != nil {
				t.Error(err)
				return
			}
			if !manager.MarkSnapshotFresh(generation) {
				t.Errorf("MarkSnapshotFresh(%d) rejected the published snapshot", generation)
				return
			}
		}
	}()

	wg.Wait()
}
