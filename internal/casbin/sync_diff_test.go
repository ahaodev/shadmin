package casbin

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"shadmin/ent"
)

// ===== 纯函数：diffKeys =====

func TestDiffKeys(t *testing.T) {
	tests := []struct {
		name             string
		current, desired []string
		wantAdd          []string
		wantRemove       []string
	}{
		{name: "both empty"},
		{
			name:    "empty current/desired non-empty",
			desired: []string{"a", "b"},
			wantAdd: []string{"a", "b"},
		},
		{
			name:       "current non-empty/empty desired",
			current:    []string{"a", "b"},
			wantRemove: []string{"a", "b"},
		},
		{
			name:    "identical",
			current: []string{"a", "b"},
			desired: []string{"a", "b"},
		},
		{
			name:       "overlap",
			current:    []string{"a", "b", "c"},
			desired:    []string{"b", "c", "d"},
			wantAdd:    []string{"d"},
			wantRemove: []string{"a"},
		},
		{
			name:       "disjoint",
			current:    []string{"a"},
			desired:    []string{"b"},
			wantAdd:    []string{"b"},
			wantRemove: []string{"a"},
		},
		{
			name:       "duplicates collapse and keep order",
			current:    []string{"a", "a", "b"},
			desired:    []string{"c", "c", "b"},
			wantAdd:    []string{"c"},
			wantRemove: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotRemove := diffKeys(tt.current, tt.desired)
			if !sameStrings(gotAdd, tt.wantAdd) {
				t.Errorf("toAdd = %v, want %v", gotAdd, tt.wantAdd)
			}
			if !sameStrings(gotRemove, tt.wantRemove) {
				t.Errorf("toRemove = %v, want %v", gotRemove, tt.wantRemove)
			}
		})
	}
}

func TestDiffKeys_NoIntersectionRuleIsTouched(t *testing.T) {
	// 不变量：交集元素既不进 toAdd 也不进 toRemove。
	// 这是"过渡态恒为并集超集、交集权限不消失"的充分条件。
	current := []string{"a", "b", "c"}
	desired := []string{"b", "c", "d"}

	toAdd, toRemove := diffKeys(current, desired)
	for _, k := range append(append([]string{}, toAdd...), toRemove...) {
		if k == "b" || k == "c" {
			t.Fatalf("intersection element %q was scheduled for mutation", k)
		}
	}
}

// ===== 纯函数：desiredRolePolicies =====

func TestDesiredRolePolicies(t *testing.T) {
	menus := []*ent.Menu{{
		Edges: ent.MenuEdges{
			APIResources: []*ent.ApiResource{
				{Path: "/api/v1/users", Method: "GET"},
				{Path: "/api/v1/users", Method: "GET"}, // 重复项应折叠
				{Path: "/api/v1/roles", Method: "POST"},
				{Path: "/healthz", Method: "GET", IsPublic: true}, // 公开资源应排除
			},
		},
	}}

	admin := desiredRolePolicies("admin", menus)
	if len(admin) != 1 || admin[0] != (policyRule{obj: "*", act: "*"}) {
		t.Fatalf("admin desired = %v, want single wildcard", admin)
	}

	want := []policyRule{
		{obj: "/api/v1/users", act: "GET"},
		{obj: "/api/v1/roles", act: "POST"},
	}
	got := desiredRolePolicies("operator", menus)
	if len(got) != len(want) {
		t.Fatalf("operator desired = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("operator desired = %v, want %v", got, want)
		}
	}
}

func TestDesiredRolePolicies_NoMenus(t *testing.T) {
	if got := desiredRolePolicies("operator", nil); len(got) != 0 {
		t.Fatalf("desired = %v, want empty", got)
	}
}

// ===== 差分应用：无真空不变量 =====

// policySnapshotManager 在每次策略增删后记录该角色的策略快照，
// 用来断言过程中任一时刻"改动前后都有效的权限"都在。
type policySnapshotManager struct {
	Manager
	snapshots [][]policyRule
}

func (m *policySnapshotManager) AddPolicy(role, obj, act string) (bool, error) {
	ok, err := m.Manager.AddPolicy(role, obj, act)
	m.snapshots = append(m.snapshots, currentRolePolicies(m.Manager, role))
	return ok, err
}

func (m *policySnapshotManager) RemovePolicy(role, obj, act string) (bool, error) {
	ok, err := m.Manager.RemovePolicy(role, obj, act)
	m.snapshots = append(m.snapshots, currentRolePolicies(m.Manager, role))
	return ok, err
}

func TestApplyRolePolicyDiff_NeverRevokesUnchanged(t *testing.T) {
	const roleID = "role-1"
	base := testManager(t)
	mustAddPolicy(t, base, roleID, "/a", "GET")
	mustAddPolicy(t, base, roleID, "/b", "GET")
	mustAddPolicy(t, base, roleID, "/c", "GET")

	sm := &policySnapshotManager{Manager: base}
	svc := &SyncService{manager: sm}

	// desired = {B, C, D}：A 撤销、D 新增，B/C 不应被触碰。
	desired := []policyRule{
		{obj: "/b", act: "GET"},
		{obj: "/c", act: "GET"},
		{obj: "/d", act: "POST"},
	}
	if err := svc.applyRolePolicyDiff(roleID, desired); err != nil {
		t.Fatalf("applyRolePolicyDiff: %v", err)
	}

	for i, snap := range sm.snapshots {
		if !containsRule(snap, policyRule{obj: "/b", act: "GET"}) ||
			!containsRule(snap, policyRule{obj: "/c", act: "GET"}) {
			t.Fatalf("snapshot %d dropped an unchanged permission: %v", i, snap)
		}
	}

	assertRuleSet(t, sm.Manager, roleID, desired)
}

func TestApplyRolePolicyDiff_RevokesAllWhenDesiredEmpty(t *testing.T) {
	const roleID = "role-1"
	base := testManager(t)
	mustAddPolicy(t, base, roleID, "/a", "GET")
	mustAddPolicy(t, base, roleID, "/b", "POST")

	svc := &SyncService{manager: base}
	if err := svc.applyRolePolicyDiff(roleID, nil); err != nil {
		t.Fatalf("applyRolePolicyDiff: %v", err)
	}
	if got := currentRolePolicies(base, roleID); len(got) != 0 {
		t.Fatalf("policies = %v, want empty after revocation", got)
	}
}

func TestApplyRolePolicyDiff_AdminWildcardSwap(t *testing.T) {
	const roleID = "role-1"
	base := testManager(t)
	mustAddPolicy(t, base, roleID, "/a", "GET")

	svc := &SyncService{manager: base}

	// 非 admin → admin：只剩通配策略。
	if err := svc.applyRolePolicyDiff(roleID, desiredRolePolicies("admin", nil)); err != nil {
		t.Fatalf("applyRolePolicyDiff(admin): %v", err)
	}
	assertRuleSet(t, base, roleID, []policyRule{{obj: "*", act: "*"}})

	// admin → 非 admin：通配被撤销，具体策略生效。
	menus := []*ent.Menu{{Edges: ent.MenuEdges{APIResources: []*ent.ApiResource{{Path: "/a", Method: "GET"}}}}}
	if err := svc.applyRolePolicyDiff(roleID, desiredRolePolicies("operator", menus)); err != nil {
		t.Fatalf("applyRolePolicyDiff(operator): %v", err)
	}
	assertRuleSet(t, base, roleID, []policyRule{{obj: "/a", act: "GET"}})
}

// roleSnapshotManager 在每次角色增删后记录该用户的角色快照。
type roleSnapshotManager struct {
	Manager
	snapshots [][]string
}

func (m *roleSnapshotManager) AddRoleForUser(user, role string) (bool, error) {
	ok, err := m.Manager.AddRoleForUser(user, role)
	m.snapshots = append(m.snapshots, m.Manager.GetRolesForUser(user))
	return ok, err
}

func (m *roleSnapshotManager) DeleteRoleForUser(user, role string) (bool, error) {
	ok, err := m.Manager.DeleteRoleForUser(user, role)
	m.snapshots = append(m.snapshots, m.Manager.GetRolesForUser(user))
	return ok, err
}

func TestApplyUserRoleDiff_NeverRevokesUnchanged(t *testing.T) {
	const userID = "user-1"
	base := testManager(t)
	for _, roleID := range []string{"r1", "r2", "r3"} {
		if _, err := base.AddRoleForUser(userID, roleID); err != nil {
			t.Fatalf("AddRoleForUser(%q): %v", roleID, err)
		}
	}

	sm := &roleSnapshotManager{Manager: base}
	svc := &SyncService{manager: sm}

	// desired = {r2, r3, r4}：r1 撤销、r4 新增，r2/r3 不应被触碰。
	if err := svc.applyUserRoleDiff(userID, []string{"r2", "r3", "r4"}); err != nil {
		t.Fatalf("applyUserRoleDiff: %v", err)
	}

	for i, snap := range sm.snapshots {
		if !containsString(snap, "r2") || !containsString(snap, "r3") {
			t.Fatalf("snapshot %d dropped an unchanged role: %v", i, snap)
		}
	}
	if got := sm.Manager.GetRolesForUser(userID); !sameStringSet(got, []string{"r2", "r3", "r4"}) {
		t.Fatalf("roles = %v, want {r2 r3 r4}", got)
	}
}

func TestApplyUserRoleDiff_RevokesAllWhenDesiredEmpty(t *testing.T) {
	const userID = "user-1"
	base := testManager(t)
	for _, roleID := range []string{"r1", "r2"} {
		if _, err := base.AddRoleForUser(userID, roleID); err != nil {
			t.Fatalf("AddRoleForUser(%q): %v", roleID, err)
		}
	}

	svc := &SyncService{manager: base}
	if err := svc.applyUserRoleDiff(userID, nil); err != nil {
		t.Fatalf("applyUserRoleDiff: %v", err)
	}
	if got := base.GetRolesForUser(userID); len(got) != 0 {
		t.Fatalf("roles = %v, want empty after revocation", got)
	}
}

// ===== 错误聚合：失败必须上抛，兜底调度才会重试 =====

var errAddBoom = errors.New("add boom")

type failingAddPolicyManager struct{ Manager }

func (m failingAddPolicyManager) AddPolicy(role, obj, act string) (bool, error) {
	return false, errAddBoom
}

type failingAddRoleManager struct{ Manager }

func (m failingAddRoleManager) AddRoleForUser(user, role string) (bool, error) {
	return false, errAddBoom
}

func TestApplyRolePolicyDiff_AggregatesErrors(t *testing.T) {
	svc := &SyncService{manager: failingAddPolicyManager{testManager(t)}}
	if err := svc.applyRolePolicyDiff("role-1", []policyRule{{obj: "/a", act: "GET"}}); !errors.Is(err, errAddBoom) {
		t.Fatalf("err = %v, want to wrap %v", err, errAddBoom)
	}
}

func TestApplyUserRoleDiff_AggregatesErrors(t *testing.T) {
	svc := &SyncService{manager: failingAddRoleManager{testManager(t)}}
	if err := svc.applyUserRoleDiff("user-1", []string{"r1"}); !errors.Is(err, errAddBoom) {
		t.Fatalf("err = %v, want to wrap %v", err, errAddBoom)
	}
}

// ===== helpers =====

func containsRule(rules []policyRule, want policyRule) bool {
	return slices.Contains(rules, want)
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func assertRuleSet(t *testing.T, m Manager, roleID string, want []policyRule) {
	t.Helper()
	got := currentRolePolicies(m, roleID)
	if len(got) != len(want) {
		t.Fatalf("policies for %s = %v, want %v", roleID, got, want)
	}
	for _, w := range want {
		if !containsRule(got, w) {
			t.Fatalf("policies for %s = %v, want %v", roleID, got, want)
		}
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !containsString(got, w) {
			return false
		}
	}
	return true
}

// ===== 并发：差集读改写必须互斥 =====

// TestApplyUserRoleDiff_ConcurrentWritersDoNotInterleave 覆盖 hook worker 与定时调度器
// 同时为同一用户同步角色的场景：并发执行读改写会让两次写入交错，留下双方目标集的并集。
// 有了内部互斥，最终状态必须正好等于其中一次的目标集。
func TestApplyUserRoleDiff_ConcurrentWritersDoNotInterleave(t *testing.T) {
	const userID = "user-1"
	svc := &SyncService{manager: testManager(t)}

	desiredSets := [][]string{{"r1"}, {"r2"}, {"r3"}, {"r4"}}

	var wg sync.WaitGroup
	for _, desired := range desiredSets {
		wg.Add(1)
		go func(desired []string) {
			defer wg.Done()
			for range 50 {
				if err := svc.applyUserRoleDiff(userID, desired); err != nil {
					t.Errorf("applyUserRoleDiff(%v): %v", desired, err)
					return
				}
			}
		}(desired)
	}
	wg.Wait()

	got := svc.manager.GetRolesForUser(userID)
	if len(got) != 1 {
		t.Fatalf("roles = %v, want exactly one writer's desired set (interleaved writes?)", got)
	}
	for _, desired := range desiredSets {
		if sameStringSet(got, desired) {
			return
		}
	}
	t.Fatalf("roles = %v, not equal to any concurrent desired set", got)
}

// 单个角色的差分同步不得触碰其他角色的策略（过滤读只取本角色的规则）。
func TestApplyRolePolicyDiff_LeavesOtherRolesUntouched(t *testing.T) {
	base := testManager(t)
	mustAddPolicy(t, base, "role-1", "/a", "GET")
	mustAddPolicy(t, base, "role-2", "/b", "POST")
	mustAddPolicy(t, base, "role-2", "/c", "POST")

	svc := &SyncService{manager: base}
	// role-1 收敛到空集：只应撤销 role-1 自己的策略。
	if err := svc.applyRolePolicyDiff("role-1", nil); err != nil {
		t.Fatalf("applyRolePolicyDiff: %v", err)
	}

	if got := currentRolePolicies(base, "role-1"); len(got) != 0 {
		t.Fatalf("role-1 policies = %v, want empty", got)
	}
	assertRuleSet(t, base, "role-2", []policyRule{{obj: "/b", act: "POST"}, {obj: "/c", act: "POST"}})
}
