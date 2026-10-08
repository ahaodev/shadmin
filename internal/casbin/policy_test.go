package casbin

import (
	"testing"

	"shadmin/domain"
)

func TestDesiredRolePolicies(t *testing.T) {
	resources := []domain.AuthorizedResource{
		{Path: "/api/v1/system/user", Method: "GET"},
		{Path: "/api/v1/system/user", Method: "GET"},
		{Path: "/api/v1/health", Method: "GET", IsPublic: true},
	}

	got := desiredRolePolicies("dev", resources)
	want := []policyRule{{obj: "/api/v1/system/user", act: "GET"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("desiredRolePolicies = %v, want %v", got, want)
	}
}

func TestDesiredRolePolicies_DevRoleForAhao(t *testing.T) {
	const (
		userID = "ahao"
		roleID = "dev"
	)
	resources := []domain.AuthorizedResource{
		{Path: "/api/v1/system/user", Method: "GET"},
		{Path: "/api/v1/system/user", Method: "POST"},
		{Path: "/api/v1/system/user/:id", Method: "GET"},
		{Path: "/api/v1/system/role", Method: "GET"},
		{Path: "/api/v1/system/role/:id", Method: "PUT"},
	}

	enforcer, err := newEnforcer()
	if err != nil {
		t.Fatalf("newEnforcer: %v", err)
	}
	if _, err := enforcer.AddRoleForUser(userID, roleID); err != nil {
		t.Fatalf("AddRoleForUser: %v", err)
	}
	for _, rule := range desiredRolePolicies(roleID, resources) {
		if _, err := enforcer.AddNamedPolicy("p", roleID, rule.obj, rule.act); err != nil {
			t.Fatalf("AddNamedPolicy(%q, %q): %v", rule.obj, rule.act, err)
		}
	}

	manager := &CasManager{
		snapshot: &authorizationSnapshot{enforcer: enforcer, generation: 0},
	}
	assertPermission(t, manager, userID, "/api/v1/system/user", "GET", true)
	assertPermission(t, manager, userID, "/api/v1/system/user", "POST", true)
	assertPermission(t, manager, userID, "/api/v1/system/user/42", "GET", true)
	assertPermission(t, manager, userID, "/api/v1/system/role", "GET", true)
	assertPermission(t, manager, userID, "/api/v1/system/role/42", "PUT", true)

	assertPermission(t, manager, userID, "/api/v1/system/user/42", "DELETE", false)
	assertPermission(t, manager, userID, "/api/v1/system/role/42", "DELETE", false)
	assertPermission(t, manager, userID, "/api/v1/system/menu", "GET", false)
}

func TestDesiredRolePolicies_AdminWildcard(t *testing.T) {
	got := desiredRolePolicies("admin", nil)
	want := []policyRule{{obj: "*", act: "*"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("desiredRolePolicies(admin) = %v, want %v", got, want)
	}
}
