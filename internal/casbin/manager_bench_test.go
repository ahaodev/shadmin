package casbin

import (
	"fmt"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinlog "github.com/casbin/casbin/v3/log"
)

type silentLogger struct{}

func (silentLogger) SetEventTypes([]casbinlog.EventType) error            { return nil }
func (silentLogger) OnBeforeEvent(*casbinlog.LogEntry) error              { return nil }
func (silentLogger) OnAfterEvent(*casbinlog.LogEntry) error               { return nil }
func (silentLogger) SetLogCallback(func(*casbinlog.LogEntry) error) error { return nil }

var benchRoles = []string{"bench-admin", "bench-ops", "bench-auditor", "bench-viewer", "bench-developer", "bench-support", "bench-finance", "bench-guest"}

const benchUser = "bench-user"
const benchObj = "/api/v1/system/bench-res119/:id"

func setupBenchPolicy(tb testing.TB) (*CasManager, *casbin.SyncedEnforcer) {
	tb.Helper()
	m, _ := testManagerWithEnforcer(tb, func(e *casbin.SyncedEnforcer) {
		for _, role := range benchRoles {
			for i := range 120 {
				obj := fmt.Sprintf("/api/v1/system/bench-res%03d/:id", i)
				mustAddPolicy(tb, e, role, obj, "GET")
				mustAddPolicy(tb, e, role, obj, "POST")
			}
		}
		mustAddPolicy(tb, e, "bench-role-wild", "*", "*")
		mustAddRoleForUser(tb, e, benchUser, "bench-admin")
		mustAddRoleForUser(tb, e, benchUser, "bench-role-wild")
	})
	e := m.currentSnapshot().enforcer

	if allowed, err := m.CheckPermission(benchUser, benchObj, "GET"); err != nil || !allowed {
		tb.Fatalf("warmup CheckPermission: allowed=%v err=%v", allowed, err)
	}
	return m, e
}

func BenchmarkCheckPermission_AllowAssignedRole(b *testing.B) {
	m, _ := setupBenchPolicy(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allowed, err := m.CheckPermission(benchUser, benchObj, "GET")
		if err != nil {
			b.Fatalf("err=%v", err)
		}
		if !allowed {
			b.Fatal("allowed=false, want true")
		}
	}
}

func BenchmarkCheckPermission_AllowAssignedRoleParallel(b *testing.B) {
	m, _ := setupBenchPolicy(b)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			allowed, err := m.CheckPermission(benchUser, benchObj, "GET")
			if err != nil {
				b.Errorf("err=%v", err)
				return
			}
			if !allowed {
				b.Errorf("allowed=false, want true")
				return
			}
		}
	})
}

// 无匹配 subject，测量完整策略扫描的拒绝开销。
func BenchmarkEnforce_DenyNoMatchingSubject(b *testing.B) {
	_, e := setupBenchPolicy(b)
	e.SetLogger(silentLogger{}) // deny 路径每迭代打一条告警日志，静默后才可比

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allowed, err := e.Enforce("bench-user-without-any-policy", benchObj, "GET")
		if err != nil {
			b.Fatal(err)
		}
		if allowed {
			b.Fatal("expected deny")
		}
	}
}
