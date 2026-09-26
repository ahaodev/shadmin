package bootstrap

import (
	"context"
	"testing"

	"shadmin/ent/role"
	"shadmin/ent/user"
	"shadmin/internal/conf"
	"shadmin/internal/constants"
	"shadmin/repository"
)

func TestInitDefaultAdminRepairsMissingAdminRole(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}
	admin, err := client.User.Create().
		SetUsername("admin").
		SetStatus(user.StatusActive).
		SetIsAdmin(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create admin user without role: %v", err)
	}
	if err := client.Menu.Create().
		SetName("existing-menu").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		Exec(ctx); err != nil {
		t.Fatalf("create existing menu: %v", err)
	}

	if err := InitDefaultAdmin(&Application{DB: client}); err != nil {
		t.Fatalf("InitDefaultAdmin: %v", err)
	}
	adminRole, err := client.Role.Query().Where(role.NameEQ("admin")).Only(ctx)
	if err != nil {
		t.Fatalf("query repaired admin role: %v", err)
	}
	assigned, err := client.User.Get(ctx, admin.ID)
	if err != nil {
		t.Fatalf("reload admin user: %v", err)
	}
	roleIDs, err := assigned.QueryRoles().IDs(ctx)
	if err != nil {
		t.Fatalf("query repaired admin assignment: %v", err)
	}
	if len(roleIDs) != 1 || roleIDs[0] != adminRole.ID {
		t.Fatalf("admin role IDs = %v, want [%s]", roleIDs, adminRole.ID)
	}
}

func TestInitDefaultAdminBindsExistingMenusWhenCreatingAdmin(t *testing.T) {
	client := newTestEntClient(t)
	if err := repository.EnsureAuthorizationState(context.Background(), client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}
	if err := client.Menu.Create().
		SetName("existing-menu").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		Exec(context.Background()); err != nil {
		t.Fatalf("create existing menu: %v", err)
	}

	app := &Application{DB: client, Env: &conf.Env{AdminUsername: "admin", AdminPassword: "unused", AdminEmail: "admin@example.com"}}
	if err := InitDefaultAdmin(app); err != nil {
		t.Fatalf("InitDefaultAdmin: %v", err)
	}
	menuCount, err := client.Role.Query().Where(role.NameEQ("admin")).QueryMenus().Count(context.Background())
	if err != nil || menuCount != 1 {
		t.Fatalf("new admin menu count = %d, err = %v; want 1", menuCount, err)
	}
}

func TestInitDefaultAdminBumpsGenerationWhenBindingMenus(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	adminRole, err := client.Role.Create().
		SetName("admin").
		SetIsSystem(true).
		SetStatus(constants.StatusActive).
		Save(ctx)
	if err != nil {
		t.Fatalf("create admin role: %v", err)
	}
	if err := client.User.Create().
		SetUsername("admin").
		SetStatus(user.StatusActive).
		SetIsAdmin(true).
		AddRoles(adminRole).
		Exec(ctx); err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	if err := client.Menu.Create().
		SetName("dashboard").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		Exec(ctx); err != nil {
		t.Fatalf("create menu: %v", err)
	}

	app := &Application{DB: client, Env: &conf.Env{AdminPassword: "unused"}}
	if err := InitDefaultAdmin(app); err != nil {
		t.Fatalf("InitDefaultAdmin: %v", err)
	}
	if generation, err := repository.CurrentAuthorizationGeneration(ctx, client); err != nil || generation != 1 {
		t.Fatalf("authorization generation = %d, err = %v; want 1", generation, err)
	}
	menuCount, err := client.Role.Query().Where(role.IDEQ(adminRole.ID)).QueryMenus().Count(ctx)
	if err != nil || menuCount != 1 {
		t.Fatalf("admin menu count = %d, err = %v; want 1", menuCount, err)
	}
}
