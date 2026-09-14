package casbin

import (
	"context"
	"errors"
	"testing"

	"shadmin/ent"
)

func TestSyncStatsIsHealthy(t *testing.T) {
	tests := []struct {
		name  string
		stats SyncStats
		want  bool
	}{
		{
			name:  "fresh install: both sides empty",
			stats: SyncStats{},
			want:  true,
		},
		{
			name:  "db empty but casbin has leftovers",
			stats: SyncStats{CasbinPolicies: 1},
			want:  false,
		},
		{
			// Casbin 为空不等于没同步：角色可能压根没有可投影的权限。
			// 这条曾经报 false，导致每次启动/每小时一条误报 WARN。
			name:  "roles exist but project to nothing",
			stats: SyncStats{DatabaseRoles: 1},
			want:  true,
		},
		{
			name:  "db roles projected to casbin",
			stats: SyncStats{DatabaseRoles: 3, CasbinPolicies: 7},
			want:  true,
		},
		{
			name:  "only user-role assignments, no policies yet",
			stats: SyncStats{DatabaseUserRoles: 2, CasbinRoles: 2},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stats.IsHealthy(); got != tt.want {
				t.Fatalf("IsHealthy() = %v, want %v (stats: %+v)", got, tt.want, tt.stats)
			}
		})
	}
}

func TestUniqueRoleIDsDeduplicates(t *testing.T) {
	menus := []*ent.Menu{
		{Edges: ent.MenuEdges{Roles: []*ent.Role{{ID: "r1"}, {ID: "r2"}}}},
		{Edges: ent.MenuEdges{Roles: []*ent.Role{{ID: "r2"}, {ID: ""}}}},
		{Edges: ent.MenuEdges{}},
	}

	got := uniqueRoleIDs(menus)
	if !sameStringSet(got, []string{"r1", "r2"}) {
		t.Fatalf("uniqueRoleIDs = %v, want {r1 r2}", got)
	}
}

func TestClearCasbinPolicies(t *testing.T) {
	m := testManager(t)
	mustAddPolicy(t, m, "r1", "/a", "GET")
	mustAddPolicy(t, m, "r2", "/b", "POST")
	mustAddRoleForUser(t, m, "u1", "r1")
	mustAddRoleForUser(t, m, "u2", "r2")

	svc := &SyncService{manager: m}
	if err := svc.clearCasbinPolicies(); err != nil {
		t.Fatalf("clearCasbinPolicies: %v", err)
	}

	if got := m.GetAllPolicies(); len(got) != 0 {
		t.Fatalf("policies remain after clear: %v", got)
	}
	if got := m.GetAllRoles(); len(got) != 0 {
		t.Fatalf("role mappings remain after clear: %v", got)
	}
}

var errRemoveBoom = errors.New("remove boom")

type failingRemoveFilteredPolicyManager struct{ Manager }

func (m failingRemoveFilteredPolicyManager) RemoveFilteredPolicy(_ int, _ ...string) (bool, error) {
	return false, errRemoveBoom
}

func TestClearCasbinPoliciesAggregatesErrors(t *testing.T) {
	m := testManager(t)
	mustAddPolicy(t, m, "r1", "/a", "GET")

	svc := &SyncService{manager: failingRemoveFilteredPolicyManager{m}}
	if err := svc.clearCasbinPolicies(); !errors.Is(err, errRemoveBoom) {
		t.Fatalf("err = %v, want to wrap %v", err, errRemoveBoom)
	}
}

// saveRecordingManager 让清理失败，并记录 SavePolicy 是否被调用：
// 清理失败必须中止在全量重建的写回之前，否则内存里没删掉的旧规则会被整体覆盖回存储，
// 让"全量重建"变成把幽灵权限永久化。
type saveRecordingManager struct {
	Manager
	saveCalls int
}

func (m *saveRecordingManager) RemoveFilteredPolicy(_ int, _ ...string) (bool, error) {
	return false, errRemoveBoom
}

func (m *saveRecordingManager) SavePolicy() error {
	m.saveCalls++
	return nil
}

func TestSyncFromDatabaseAbortsOnClearFailure(t *testing.T) {
	base := testManager(t)
	mustAddPolicy(t, base, "r1", "/a", "GET")

	m := &saveRecordingManager{Manager: base}
	// entClient 故意留 nil：清理失败必须在触碰 DB 之前返回，否则这里会 panic。
	svc := &SyncService{manager: m}

	err := svc.SyncFromDatabase(context.Background())
	if !errors.Is(err, errRemoveBoom) {
		t.Fatalf("err = %v, want to wrap %v", err, errRemoveBoom)
	}
	if m.saveCalls != 0 {
		t.Fatalf("SavePolicy called %d times after a failed clear, want 0", m.saveCalls)
	}
	if got := base.GetAllPolicies(); len(got) != 1 {
		t.Fatalf("policies = %v, want the pre-existing rule left untouched", got)
	}
}
