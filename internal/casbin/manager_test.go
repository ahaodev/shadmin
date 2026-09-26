package casbin

import (
	"errors"
	"testing"

	"github.com/casbin/casbin/v3"
)

func testManagerWithEnforcer(tb testing.TB, configure func(*casbin.SyncedEnforcer)) (*CasManager, *casbin.SyncedEnforcer) {
	tb.Helper()
	enforcer, err := newEnforcer()
	if err != nil {
		tb.Fatalf("newEnforcer: %v", err)
	}
	if configure != nil {
		configure(enforcer)
	}
	manager := &CasManager{
		snapshot: &authorizationSnapshot{enforcer: enforcer, generation: 0},
	}
	return manager, enforcer
}

func replaceTestSnapshot(tb testing.TB, manager *CasManager, generation int64, configure func(*casbin.SyncedEnforcer)) {
	tb.Helper()
	enforcer, err := newEnforcer()
	if err != nil {
		tb.Fatalf("newEnforcer: %v", err)
	}
	if configure != nil {
		configure(enforcer)
	}
	if err := manager.ReplaceSnapshot(enforcer, generation); err != nil {
		tb.Fatalf("ReplaceSnapshot: %v", err)
	}
	if !manager.MarkSnapshotFresh(generation) {
		tb.Fatalf("MarkSnapshotFresh(%d) rejected the published snapshot", generation)
	}
}

func mustAddPolicy(tb testing.TB, enforcer *casbin.SyncedEnforcer, role, object, action string) {
	tb.Helper()
	if _, err := enforcer.AddNamedPolicy("p", role, object, action); err != nil {
		tb.Fatalf("AddNamedPolicy(%q, %q, %q): %v", role, object, action, err)
	}
}

func mustAddRoleForUser(tb testing.TB, enforcer *casbin.SyncedEnforcer, userID, role string) {
	tb.Helper()
	if _, err := enforcer.AddRoleForUser(userID, role); err != nil {
		tb.Fatalf("AddRoleForUser(%q, %q): %v", userID, role, err)
	}
}

func assertPermission(t *testing.T, manager Manager, userID, object, action string, want bool) {
	t.Helper()
	got, err := manager.CheckPermission(userID, object, action)
	if err != nil {
		t.Fatalf("CheckPermission(%q, %q, %q): %v", userID, object, action, err)
	}
	if got != want {
		t.Fatalf("CheckPermission(%q, %q, %q) = %v, want %v", userID, object, action, got, want)
	}
}

func TestNewCasManagerStartsStale(t *testing.T) {
	manager := NewCasManager()
	if allowed, err := manager.CheckPermission("user", "/api/test", "GET"); allowed || !errors.Is(err, ErrSnapshotStale) {
		t.Fatalf("CheckPermission before first snapshot = (%v, %v), want (false, ErrSnapshotStale)", allowed, err)
	}
}

func TestManagerStaleSnapshotFailsClosedAndCanRecover(t *testing.T) {
	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, "user", "role")
		mustAddPolicy(t, enforcer, "role", "/api/test", "GET")
	})

	manager.MarkSnapshotStale()
	if allowed, err := manager.CheckPermission("user", "/api/test", "GET"); allowed || !errors.Is(err, ErrSnapshotStale) {
		t.Fatalf("CheckPermission while stale = (%v, %v), want (false, ErrSnapshotStale)", allowed, err)
	}
	if !manager.MarkSnapshotFresh(0) {
		t.Fatal("MarkSnapshotFresh rejected the current generation")
	}
	assertPermission(t, manager, "user", "/api/test", "GET", true)
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

	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddPolicy(t, enforcer, roleExact, objExact, "GET")
		mustAddPolicy(t, enforcer, roleExact, "/api/v1/res/:id", "POST")
		mustAddPolicy(t, enforcer, roleWild, "*", "*")
		mustAddRoleForUser(t, enforcer, userNormal, roleExact)
		mustAddRoleForUser(t, enforcer, userWild, roleWild)
	})

	cases := []struct {
		name                   string
		userID, object, action string
		want                   bool
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
			assertPermission(t, manager, tc.userID, tc.object, tc.action, tc.want)
		})
	}
}

func TestCheckPermission_GrantTakesEffectAfterSnapshotPublication(t *testing.T) {
	const userID, roleID, object = "user", "role", "/api/v1/res"
	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
	})
	assertPermission(t, manager, userID, object, "GET", false)

	replaceTestSnapshot(t, manager, 1, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
		mustAddPolicy(t, enforcer, roleID, object, "GET")
	})
	assertPermission(t, manager, userID, object, "GET", true)
}

func TestCheckPermission_RevokeTakesEffectAfterSnapshotPublication(t *testing.T) {
	const userID, roleID, object = "user", "role", "/api/v1/res"
	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
		mustAddPolicy(t, enforcer, roleID, object, "GET")
	})
	assertPermission(t, manager, userID, object, "GET", true)

	replaceTestSnapshot(t, manager, 1, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
	})
	assertPermission(t, manager, userID, object, "GET", false)
}

func TestCheckPermission_RoleRemovalTakesEffectAfterSnapshotPublication(t *testing.T) {
	const userID, roleID, object = "user", "role", "/api/v1/res"
	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
		mustAddPolicy(t, enforcer, roleID, object, "GET")
	})
	assertPermission(t, manager, userID, object, "GET", true)

	replaceTestSnapshot(t, manager, 1, func(enforcer *casbin.SyncedEnforcer) {
		mustAddPolicy(t, enforcer, roleID, object, "GET")
	})
	assertPermission(t, manager, userID, object, "GET", false)
}

func TestCheckPermission_StableAcrossRepeatedCalls(t *testing.T) {
	const userID, roleID, object = "user", "role", "/api/v1/res"
	manager, _ := testManagerWithEnforcer(t, func(enforcer *casbin.SyncedEnforcer) {
		mustAddRoleForUser(t, enforcer, userID, roleID)
		mustAddPolicy(t, enforcer, roleID, object, "GET")
	})

	for i := range 200 {
		allowed, err := manager.CheckPermission(userID, object, "GET")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call %d: got deny, want allow", i)
		}
	}
}
