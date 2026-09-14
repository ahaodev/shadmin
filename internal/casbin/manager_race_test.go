package casbin

import (
	"sync"
	"testing"
)

// TestManagerConcurrentReadWrite 守护 CasManager 的读并发安全：
// API 中间件的 CheckPermission（读）与同步 worker 的策略写入（写）必然并发，
// 底层 casbin.Enforcer 无内部锁，必须使用 SyncedEnforcer。
func TestManagerConcurrentReadWrite(t *testing.T) {
	m := testManager(t)
	mustAddRoleForUser(t, m, "u1", "r1")
	mustAddPolicy(t, m, "r1", "/api/x", "GET")

	const iterations = 2000

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range iterations {
			if _, err := m.CheckPermission("u1", "/api/x", "GET"); err != nil {
				t.Error(err)
				return
			}
			if roles := m.GetRolesForUser("u1"); len(roles) != 1 {
				t.Errorf("roles = %v, want [r1]", roles)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for range iterations {
			if _, err := m.AddPolicy("r1", "/api/y", "POST"); err != nil {
				t.Error(err)
				return
			}
			if _, err := m.RemovePolicy("r1", "/api/y", "POST"); err != nil {
				t.Error(err)
				return
			}
		}
	}()

	wg.Wait()
}
