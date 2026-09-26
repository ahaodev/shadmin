package bootstrap

import (
	"context"
	"testing"

	"shadmin/domain"
	"shadmin/ent"
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
	viewerRole, err := client.Role.Query().Where(role.Name(domain.RoleNameViewer)).Only(ctx)
	if err != nil {
		t.Fatalf("query initialized viewer role: %v", err)
	}
	if !viewerRole.IsSystem || viewerRole.Status != constants.StatusActive {
		t.Fatalf("viewer role = system:%v status:%q, want system and active", viewerRole.IsSystem, viewerRole.Status)
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

func TestEnsureViewerRoleHasOnlyReadOnlyPageMenus(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	dictID := "GET:/api/v1/system/dict/types"
	postID := "POST:/api/v1/system/dict/types"
	userID := "GET:/api/v1/system/user"
	roleID := "GET:/api/v1/system/role"
	apiResourceID := "GET:/api/v1/system/api-resources"
	for _, resource := range []struct{ id, method, path string }{
		{dictID, "GET", "/api/v1/system/dict/types"},
		{postID, "POST", "/api/v1/system/dict/types"},
		{userID, "GET", "/api/v1/system/user"},
		{roleID, "GET", "/api/v1/system/role"},
		{apiResourceID, "GET", "/api/v1/system/api-resources"},
	} {
		if err := client.ApiResource.Create().
			SetID(resource.id).
			SetMethod(resource.method).
			SetPath(resource.path).
			SetHandler("test.handler").
			Exec(ctx); err != nil {
			t.Fatalf("create API resource %s: %v", resource.id, err)
		}
	}

	dictMenu, err := client.Menu.Create().
		SetName("字典管理").
		SetPath("/system/dict").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		AddAPIResourceIDs(dictID).
		Save(ctx)
	if err != nil {
		t.Fatalf("create dictionary page menu: %v", err)
	}
	userMenu, err := client.Menu.Create().
		SetName("用户管理").
		SetPath("/system/user").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		AddAPIResourceIDs(userID).
		Save(ctx)
	if err != nil {
		t.Fatalf("create user page menu: %v", err)
	}
	roleMenu, err := client.Menu.Create().
		SetName("角色管理").
		SetPath("/system/role").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		AddAPIResourceIDs(roleID).
		Save(ctx)
	if err != nil {
		t.Fatalf("create role page menu: %v", err)
	}
	apiResourceMenu, err := client.Menu.Create().
		SetName("API资源").
		SetPath("/system/api-resources").
		SetType("menu").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		AddAPIResourceIDs(apiResourceID).
		Save(ctx)
	if err != nil {
		t.Fatalf("create API resource page menu: %v", err)
	}
	buttonMenu, err := client.Menu.Create().
		SetName("创建字典").
		SetParentID(dictMenu.ID).
		SetType("button").
		SetVisible("show").
		SetStatus(constants.StatusActive).
		AddAPIResourceIDs(postID).
		Save(ctx)
	if err != nil {
		t.Fatalf("create dictionary action menu: %v", err)
	}

	if err := ensureViewerRole(ctx, client); err != nil {
		t.Fatalf("ensure viewer role: %v", err)
	}
	viewerRole, err := client.Role.Query().
		Where(role.Name(domain.RoleNameViewer)).
		WithMenus().
		Only(ctx)
	if err != nil {
		t.Fatalf("query viewer role: %v", err)
	}
	if !viewerRole.IsSystem || viewerRole.Status != constants.StatusActive {
		t.Fatalf("viewer role = system:%v status:%q, want system and active", viewerRole.IsSystem, viewerRole.Status)
	}
	wantMenuIDs := map[string]struct{}{
		dictMenu.ID:        {},
		apiResourceMenu.ID: {},
		roleMenu.ID:        {},
		userMenu.ID:        {},
	}
	if len(viewerRole.Edges.Menus) != len(wantMenuIDs) {
		t.Fatalf("viewer menu IDs = %v, want dictionary and API resource menus", menuIDs(viewerRole.Edges.Menus))
	}
	for _, menu := range viewerRole.Edges.Menus {
		if _, ok := wantMenuIDs[menu.ID]; !ok {
			t.Fatalf("unexpected viewer menu ID %q", menu.ID)
		}
		apiResources, err := menu.QueryAPIResources().All(ctx)
		if err != nil {
			t.Fatalf("query viewer menu API resources: %v", err)
		}
		if len(apiResources) == 0 {
			t.Fatalf("viewer menu %q has no read API resources", menu.Name)
		}
		for _, apiResource := range apiResources {
			if apiResource.Method != "GET" {
				t.Fatalf("viewer menu %q includes non-read method %q", menu.Name, apiResource.Method)
			}
		}
	}
	if needsRepair, err := needsViewerRoleInitialization(ctx, client); err != nil || needsRepair {
		t.Fatalf("viewer role needs repair = %v, err = %v; want false", needsRepair, err)
	}

	if _, err := viewerRole.Update().AddMenus(buttonMenu).Save(ctx); err != nil {
		t.Fatalf("add unsafe viewer menu: %v", err)
	}
	if needsRepair, err := needsViewerRoleInitialization(ctx, client); err != nil || !needsRepair {
		t.Fatalf("viewer role needs repair = %v, err = %v; want true", needsRepair, err)
	}
	if err := ensureViewerRole(ctx, client); err != nil {
		t.Fatalf("repair viewer role: %v", err)
	}
	viewerRole, err = client.Role.Query().Where(role.Name(domain.RoleNameViewer)).WithMenus().Only(ctx)
	if err != nil {
		t.Fatalf("reload viewer role: %v", err)
	}
	if len(viewerRole.Edges.Menus) != len(wantMenuIDs) {
		t.Fatalf("repaired viewer menu IDs = %v, want dictionary and API resource menus", menuIDs(viewerRole.Edges.Menus))
	}
	for _, menu := range viewerRole.Edges.Menus {
		if _, ok := wantMenuIDs[menu.ID]; !ok {
			t.Fatalf("unexpected repaired viewer menu ID %q", menu.ID)
		}
	}
}

func menuIDs(menus []*ent.Menu) []string {
	ids := make([]string, len(menus))
	for i, menu := range menus {
		ids[i] = menu.ID
	}
	return ids
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
