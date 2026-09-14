package casbin

import (
	"fmt"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinlog "github.com/casbin/casbin/v3/log"
)

// silentLogger 抑制 casbin 的 deny 告警日志，避免 benchmark 输出刷屏。
type silentLogger struct{}

func (silentLogger) SetEventTypes([]casbinlog.EventType) error            { return nil }
func (silentLogger) OnBeforeEvent(*casbinlog.LogEntry) error              { return nil }
func (silentLogger) OnAfterEvent(*casbinlog.LogEntry) error               { return nil }
func (silentLogger) SetLogCallback(func(*casbinlog.LogEntry) error) error { return nil }

// 模拟中等规模的 RBAC 策略集：8 个角色 × 120 个接口 × 2 个方法。
var benchRoles = []string{"bench-admin", "bench-ops", "bench-auditor", "bench-viewer", "bench-developer", "bench-support", "bench-finance", "bench-guest"}

const benchUser = "bench-user"
const benchObj = "/api/v1/system/bench-res119/:id"

// setupBenchPolicy 为每个 benchmark 构建独立的策略集并预热一次，
func setupBenchPolicy(tb testing.TB) (Manager, *casbin.SyncedEnforcer) {
	tb.Helper()
	m, e := testManagerWithEnforcer(tb)

	for _, role := range benchRoles {
		for i := range 120 {
			obj := fmt.Sprintf("/api/v1/system/bench-res%03d/:id", i)
			if _, err := m.AddPolicy(role, obj, "GET"); err != nil {
				tb.Fatalf("AddPolicy: %v", err)
			}
			if _, err := m.AddPolicy(role, obj, "POST"); err != nil {
				tb.Fatalf("AddPolicy: %v", err)
			}
		}
	}
	if _, err := m.AddPolicy("bench-role-wild", "*", "*"); err != nil {
		tb.Fatalf("AddPolicy: %v", err)
	}
	if _, err := m.AddRoleForUser(benchUser, "bench-admin"); err != nil {
		tb.Fatalf("AddRoleForUser: %v", err)
	}
	if _, err := m.AddRoleForUser(benchUser, "bench-role-wild"); err != nil {
		tb.Fatalf("AddRoleForUser: %v", err)
	}

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

// 直接调用 Enforce 绕开 CheckPermission 的角色查询，且该 subject 无任何策略，
// 必须扫完整个策略集才能得出 deny —— 这是权限结果缓存要消除的开销。
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
