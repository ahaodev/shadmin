package casbin

import (
	"sort"
	"strings"
	"testing"

	"github.com/casbin/casbin/v3/model"
)

// memAdapter 是测试用的最小适配器：enforcer 的策略状态完全在内存中，
// 适配器只用来满足 casbin.NewEnforcer 的接口要求，不承载任何断言。
type memAdapter struct{}

func (a *memAdapter) LoadPolicy(model.Model) error                              { return nil }
func (a *memAdapter) SavePolicy(model.Model) error                              { return nil }
func (a *memAdapter) AddPolicy(string, string, []string) error                  { return nil }
func (a *memAdapter) RemovePolicy(string, string, []string) error               { return nil }
func (a *memAdapter) RemoveFilteredPolicy(string, string, int, ...string) error { return nil }

// testManager 为每个用例构建独立的 enforcer，用例之间互不污染，
// 因此夹具无需命名空间前缀，也支持 -shuffle / -count 重复运行。
func testManager(tb testing.TB) Manager {
	tb.Helper()
	e, err := newEnforcer(&memAdapter{})
	if err != nil {
		tb.Fatalf("newEnforcer: %v", err)
	}
	return &CasManager{enforcer: e}
}

func mustAddPolicy(t *testing.T, m Manager, role, obj, act string) {
	t.Helper()
	ok, err := m.AddPolicy(role, obj, act)
	if err != nil {
		t.Fatalf("AddPolicy(%q, %q, %q): %v", role, obj, act, err)
	}
	if !ok {
		t.Fatalf("AddPolicy(%q, %q, %q): ok=false, want true", role, obj, act)
	}
}

func mustRemovePolicy(t *testing.T, m Manager, role, obj, act string) {
	t.Helper()
	ok, err := m.RemovePolicy(role, obj, act)
	if err != nil {
		t.Fatalf("RemovePolicy(%q, %q, %q): %v", role, obj, act, err)
	}
	if !ok {
		t.Fatalf("RemovePolicy(%q, %q, %q): ok=false, want true", role, obj, act)
	}
}

func mustRemoveFilteredPolicy(t *testing.T, m Manager, fieldIndex int, values ...string) {
	t.Helper()
	ok, err := m.RemoveFilteredPolicy(fieldIndex, values...)
	if err != nil {
		t.Fatalf("RemoveFilteredPolicy(%d, %v): %v", fieldIndex, values, err)
	}
	if !ok {
		t.Fatalf("RemoveFilteredPolicy(%d, %v): ok=false, want true", fieldIndex, values)
	}
}

func mustAddRoleForUser(t *testing.T, m Manager, user, role string) {
	t.Helper()
	ok, err := m.AddRoleForUser(user, role)
	if err != nil {
		t.Fatalf("AddRoleForUser(%q, %q): %v", user, role, err)
	}
	if !ok {
		t.Fatalf("AddRoleForUser(%q, %q): ok=false, want true", user, role)
	}
}

func mustDeleteRolesForUser(t *testing.T, m Manager, user string) {
	t.Helper()
	ok, err := m.DeleteRolesForUser(user)
	if err != nil {
		t.Fatalf("DeleteRolesForUser(%q): %v", user, err)
	}
	if !ok {
		t.Fatalf("DeleteRolesForUser(%q): ok=false, want true", user)
	}
}

func assertPermission(t *testing.T, m Manager, userID, obj, act string, want bool) {
	t.Helper()
	got, err := m.CheckPermission(userID, obj, act)
	if err != nil {
		t.Fatalf("CheckPermission(%q, %q, %q): %v", userID, obj, act, err)
	}
	if got != want {
		t.Fatalf("CheckPermission(%q, %q, %q) = %v, want %v", userID, obj, act, got, want)
	}
}

func TestCheckPermission(t *testing.T) {
	const (
		roleExact  = "role-exact"
		roleWild   = "role-wild"
		userNormal = "user-normal"
		userWild   = "user-wild"
	)
	objExact := "/api/v1/res/allowed"
	objOther := "/api/v1/res/other"

	m := testManager(t)
	mustAddPolicy(t, m, roleExact, objExact, "GET")
	mustAddPolicy(t, m, roleExact, "/api/v1/res/:id", "POST")
	mustAddPolicy(t, m, roleWild, "*", "*")
	mustAddRoleForUser(t, m, userNormal, roleExact)
	mustAddRoleForUser(t, m, userWild, roleWild)

	cases := []struct {
		name             string
		userID, obj, act string
		want             bool
	}{
		{"exact path and method", userNormal, objExact, "GET", true},
		{"keyMatch2 path", userNormal, "/api/v1/res/42", "POST", true},
		{"wrong method", userNormal, objExact, "DELETE", false},
		{"unrelated path", userNormal, objOther, "GET", false},
		{"wildcard role", userWild, objOther, "DELETE", true},
		{"user without roles", "user-nobody", objExact, "GET", false},
		{"empty user", "", objExact, "GET", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertPermission(t, m, tc.userID, tc.obj, tc.act, tc.want)
		})
	}
}

// 当前实现无结果缓存；下面四个用例锁定"策略变更立即生效"，
// 并在将来引入结果缓存时守住失效逻辑。
func TestCheckPermission_PolicyChangeTakesEffect_OnGrant(t *testing.T) {
	const (
		role = "role"
		user = "user"
	)
	obj := "/api/v1/res"

	m := testManager(t)
	mustAddRoleForUser(t, m, user, role)

	// 先确认一个 deny 结果
	assertPermission(t, m, user, obj, "GET", false)

	mustAddPolicy(t, m, role, obj, "GET")

	assertPermission(t, m, user, obj, "GET", true)
}

// 撤权后若仍放行即为安全漏洞。
func TestCheckPermission_PolicyChangeTakesEffect_OnRevoke(t *testing.T) {
	const (
		role = "role"
		user = "user"
	)
	obj := "/api/v1/res"

	m := testManager(t)
	mustAddRoleForUser(t, m, user, role)
	mustAddPolicy(t, m, role, obj, "GET")

	// 先确认一个 allow 结果
	assertPermission(t, m, user, obj, "GET", true)

	mustRemovePolicy(t, m, role, obj, "GET")

	assertPermission(t, m, user, obj, "GET", false)
}

// RemoveFilteredPolicy 是"删除某角色所有策略"的批量入口。
func TestCheckPermission_PolicyChangeTakesEffect_OnRemoveFilteredPolicy(t *testing.T) {
	const (
		role = "role"
		user = "user"
	)
	obj := "/api/v1/res"

	m := testManager(t)
	mustAddRoleForUser(t, m, user, role)
	mustAddPolicy(t, m, role, obj, "GET")
	assertPermission(t, m, user, obj, "GET", true)

	mustRemoveFilteredPolicy(t, m, 0, role)

	assertPermission(t, m, user, obj, "GET", false)
}

func TestCheckPermission_PolicyChangeTakesEffect_OnDeleteRolesForUser(t *testing.T) {
	const (
		role = "role"
		user = "user"
	)
	obj := "/api/v1/res"

	m := testManager(t)
	mustAddRoleForUser(t, m, user, role)
	mustAddPolicy(t, m, role, obj, "GET")
	assertPermission(t, m, user, obj, "GET", true)

	mustDeleteRolesForUser(t, m, user)

	assertPermission(t, m, user, obj, "GET", false)
}

func TestCheckPermission_StableAcrossRepeatedCalls(t *testing.T) {
	const (
		role = "role"
		user = "user"
	)
	obj := "/api/v1/res"

	m := testManager(t)
	mustAddRoleForUser(t, m, user, role)
	mustAddPolicy(t, m, role, obj, "GET")

	for i := range 200 {
		got, err := m.CheckPermission(user, obj, "GET")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !got {
			t.Fatalf("call %d: got deny, want allow", i)
		}
	}
}

func TestManagerGetAllPolicies(t *testing.T) {
	m := testManager(t)

	mustAddPolicy(t, m, "role-a", "/api/v1/a", "GET")
	mustAddPolicy(t, m, "role-a", "/api/v1/b", "POST")
	mustAddPolicy(t, m, "role-b", "/api/v1/a", "GET")

	want := map[string]bool{
		"role-a,/api/v1/a,GET":  true,
		"role-a,/api/v1/b,POST": true,
		"role-b,/api/v1/a,GET":  true,
	}

	got := m.GetAllPolicies()
	if len(got) != len(want) {
		t.Fatalf("GetAllPolicies: got %d policies, want %d", len(got), len(want))
	}
	for _, p := range got {
		if !want[strings.Join(p, ",")] {
			t.Errorf("GetAllPolicies: unexpected policy %v", p)
		}
	}
}

func TestManagerRoleMappingReadback(t *testing.T) {
	m := testManager(t)

	mustAddRoleForUser(t, m, "alice", "admin")
	mustAddRoleForUser(t, m, "alice", "ops")
	mustAddRoleForUser(t, m, "bob", "viewer")

	// casbin 返回角色顺序不稳定（内部 map 迭代），断言前先排序
	roles := m.GetRolesForUser("alice")
	sort.Strings(roles)
	if got := strings.Join(roles, ","); got != "admin,ops" {
		t.Errorf("GetRolesForUser(alice) = %q, want %q", got, "admin,ops")
	}
	if got := m.GetRolesForUser("nobody"); len(got) != 0 {
		t.Errorf("GetRolesForUser(nobody) = %v, want empty", got)
	}

	want := map[string]bool{
		"alice,admin": true,
		"alice,ops":   true,
		"bob,viewer":  true,
	}
	got := m.GetAllRoles()
	if len(got) != len(want) {
		t.Fatalf("GetAllRoles: got %d mappings, want %d", len(got), len(want))
	}
	for _, r := range got {
		if len(r) != 2 || !want[strings.Join(r, ",")] {
			t.Errorf("GetAllRoles: unexpected mapping %v", r)
		}
	}
}
