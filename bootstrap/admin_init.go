package bootstrap

import (
	"context"
	"fmt"
	"shadmin/ent"
	"shadmin/ent/user"
	"shadmin/internal/constants"
	"shadmin/repository"
	"time"

	"entgo.io/ent/dialect/sql"
	"golang.org/x/crypto/bcrypt"
)

type seedMenu struct {
	key        string
	parentKey  string
	name       string
	sequence   int
	typ        string
	path       string
	icon       string
	permission string
	resources  []string
}

var defaultMenus = []seedMenu{
	{key: "dashboard", name: "仪表盘", typ: "menu", path: "/", icon: "Home"},
	{key: "system", name: "系统管理", typ: "menu", path: "/", icon: "Settings"},
	{key: "menu", parentKey: "system", name: "菜单管理", sequence: 1, typ: "menu", path: "/system/menu", icon: "Menu", resources: []string{"GET:/api/v1/system/menu", "GET:/api/v1/system/menu/tree"}},
	{key: "role", parentKey: "system", name: "角色管理", sequence: 2, typ: "menu", path: "/system/role", icon: "UserCheck", resources: []string{"GET:/api/v1/system/role"}},
	{key: "user", parentKey: "system", name: "用户管理", sequence: 3, typ: "menu", path: "/system/user", icon: "Users", resources: []string{"GET:/api/v1/system/user"}},
	{key: "department", parentKey: "system", name: "部门管理", sequence: 4, typ: "menu", path: "/system/departments", icon: "Building2", resources: []string{"GET:/api/v1/system/department/tree"}},
	{key: "api-resource", parentKey: "system", name: "API资源", sequence: 5, typ: "menu", path: "/system/api-resources", icon: "Code2", resources: []string{"GET:/api/v1/system/api-resources"}},
	{key: "login-log", parentKey: "system", name: "登录日志", sequence: 6, typ: "menu", path: "/system/login-logs", icon: "Layers3", resources: []string{"GET:/api/v1/system/login-logs"}},
	{key: "dict", parentKey: "system", name: "字典管理", sequence: 7, typ: "menu", path: "/system/dict", icon: "BookMarked", resources: []string{"GET:/api/v1/system/dict/types", "GET:/api/v1/system/dict/items", "GET:/api/v1/system/dict/types/code/:code/items"}},
}

var defaultButtons = []seedMenu{
	{key: "menu-add", parentKey: "menu", name: "创建菜单", typ: "button", icon: "PlusCircle", permission: "system:menu:add", resources: []string{"POST:/api/v1/system/menu", "GET:/api/v1/system/api-resources"}},
	{key: "menu-edit", parentKey: "menu", name: "编辑菜单", typ: "button", icon: "Edit", permission: "system:menu:edit", resources: []string{"PUT:/api/v1/system/menu/:id", "GET:/api/v1/system/menu/:id", "GET:/api/v1/system/api-resources"}},
	{key: "menu-delete", parentKey: "menu", name: "删除菜单", typ: "button", icon: "Trash2", permission: "system:menu:delete", resources: []string{"DELETE:/api/v1/system/menu/:id"}},
	{key: "role-add", parentKey: "role", name: "创建角色", typ: "button", icon: "Plus", permission: "system:role:add", resources: []string{"POST:/api/v1/system/role", "GET:/api/v1/system/menu/tree"}},
	{key: "role-delete", parentKey: "role", name: "删除角色", typ: "button", icon: "Trash2", permission: "system:role:delete", resources: []string{"DELETE:/api/v1/system/role/:id"}},
	{key: "role-edit", parentKey: "role", name: "编辑角色", typ: "button", icon: "Edit", permission: "system:role:edit", resources: []string{"PUT:/api/v1/system/role/:id", "GET:/api/v1/system/role/:id", "GET:/api/v1/system/role/:id/menus", "GET:/api/v1/system/menu/tree"}},
	{key: "user-add", parentKey: "user", name: "创建用户", typ: "button", icon: "PlusCircle", permission: "system:user:add", resources: []string{"POST:/api/v1/system/user"}},
	{key: "user-invite", parentKey: "user", name: "邀请用户", typ: "button", icon: "Phone", permission: "system:user:invite", resources: []string{"POST:/api/v1/system/user/invite"}},
	{key: "user-delete", parentKey: "user", name: "删除用户", typ: "button", icon: "Trash", permission: "system:user:delete", resources: []string{"DELETE:/api/v1/system/user/:id"}},
	{key: "user-edit", parentKey: "user", name: "编辑用户", typ: "button", icon: "Edit", permission: "system:user:edit", resources: []string{"PUT:/api/v1/system/user/:id", "GET:/api/v1/system/user/:id", "GET:/api/v1/system/user/:id/roles"}},
	{key: "department-add", parentKey: "department", name: "创建部门", typ: "button", icon: "PlusCircle", permission: "system:department:add", resources: []string{"POST:/api/v1/system/department", "GET:/api/v1/system/department/tree"}},
	{key: "department-edit", parentKey: "department", name: "编辑部门", typ: "button", icon: "Edit", permission: "system:department:edit", resources: []string{"PUT:/api/v1/system/department/:id", "GET:/api/v1/system/department/:id", "GET:/api/v1/system/department/tree"}},
	{key: "department-delete", parentKey: "department", name: "删除部门", typ: "button", icon: "Trash2", permission: "system:department:delete", resources: []string{"DELETE:/api/v1/system/department/:id"}},
	{key: "login-log-clean", parentKey: "login-log", name: "清空日志", typ: "button", icon: "Trash2", permission: "system:login_log:clean", resources: []string{"DELETE:/api/v1/system/login-logs"}},
	{key: "dict-add-type", parentKey: "dict", name: "创建字典类型", typ: "button", icon: "PlusCircle", permission: "system:dict:add_type", resources: []string{"POST:/api/v1/system/dict/types"}},
	{key: "dict-edit-type", parentKey: "dict", name: "编辑字典类型", typ: "button", icon: "Edit", permission: "system:dict:edit_type", resources: []string{"PUT:/api/v1/system/dict/types/:id", "GET:/api/v1/system/dict/types/:id"}},
	{key: "dict-delete-type", parentKey: "dict", name: "删除字典类型", typ: "button", icon: "Trash2", permission: "system:dict:delete_type", resources: []string{"DELETE:/api/v1/system/dict/types/:id"}},
	{key: "dict-add-item", parentKey: "dict", name: "创建字典项", typ: "button", icon: "Plus", permission: "system:dict:add_item", resources: []string{"POST:/api/v1/system/dict/items"}},
	{key: "dict-edit-item", parentKey: "dict", name: "编辑字典项", typ: "button", icon: "Edit", permission: "system:dict:edit_item", resources: []string{"PUT:/api/v1/system/dict/items/:id", "GET:/api/v1/system/dict/items/:id"}},
	{key: "dict-delete-item", parentKey: "dict", name: "删除字典项", typ: "button", icon: "Trash", permission: "system:dict:delete_item", resources: []string{"DELETE:/api/v1/system/dict/items/:id"}},
}

func seedDefaultMenus(ctx context.Context, client *ent.Client) error {
	menuIDs := make(map[string]string, len(defaultMenus)+len(defaultButtons))
	if err := seedMenuList(ctx, client, defaultMenus, menuIDs); err != nil {
		return err
	}
	return seedMenuList(ctx, client, defaultButtons, menuIDs)
}

func seedMenuList(ctx context.Context, client *ent.Client, items []seedMenu, menuIDs map[string]string) error {
	for _, item := range items {
		parentID := ""
		if item.parentKey != "" {
			var ok bool
			parentID, ok = menuIDs[item.parentKey]
			if !ok {
				return fmt.Errorf("seed menu %q references unknown parent %q", item.name, item.parentKey)
			}
		}
		created, err := createSeedMenu(ctx, client, item, parentID)
		if err != nil {
			return fmt.Errorf("create seed menu %q: %w", item.name, err)
		}
		menuIDs[item.key] = created.ID
	}
	return nil
}

func createSeedMenu(ctx context.Context, client *ent.Client, item seedMenu, parentID string) (*ent.Menu, error) {
	builder := client.Menu.Create().
		SetName(item.name).
		SetSequence(item.sequence).
		SetType(item.typ).
		SetPath(item.path).
		SetIcon(item.icon).
		SetIsFrame(false).
		SetVisible("show").
		SetPermissions(item.permission).
		SetStatus(constants.StatusActive)
	if parentID != "" {
		builder = builder.SetParentID(parentID)
	}
	if len(item.resources) > 0 {
		builder = builder.AddAPIResourceIDs(item.resources...)
	}
	return builder.Save(ctx)
}

// InitDefaultAdmin initializes or repairs the default admin and menus in one
// transaction, including the authorization generation change.
func InitDefaultAdmin(app *Application) error {
	ctx := context.Background()
	const maxAttempts = 3

	var lastErr error
	for attempt := range maxAttempts {
		needsInitialization, err := needsDefaultAdminInitialization(ctx, app.DB)
		if err != nil {
			lastErr = err
		} else if !needsInitialization {
			return nil
		} else {
			lastErr = repository.WithAuthorizationTx(ctx, app.DB, func(txCtx context.Context, tx *ent.Tx) error {
				txApp := *app
				txApp.DB = tx.Client()
				return initDefaultAdmin(&txApp, txCtx)
			})
			if lastErr == nil {
				return nil
			}
		}

		if attempt+1 < maxAttempts {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return fmt.Errorf("initialize default admin after %d attempts: %w", maxAttempts, lastErr)
}

func needsDefaultAdminInitialization(ctx context.Context, client *ent.Client) (bool, error) {
	adminExists, err := client.User.Query().Where(user.IsAdminEQ(true)).Exist(ctx)
	if err != nil {
		return true, fmt.Errorf("check admin user: %w", err)
	}
	if !adminExists {
		return true, nil
	}
	adminRole, err := client.Role.Query().Where(func(s *sql.Selector) {
		s.Where(sql.EQ("name", "admin"))
	}).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// initDefaultAdmin repairs the missing system role and rebinds the
			// existing administrator instead of creating a second admin user.
			return true, nil
		}
		return true, fmt.Errorf("get admin role: %w", err)
	}
	menuCount, err := client.Role.Query().
		Where(func(s *sql.Selector) { s.Where(sql.EQ("id", adminRole.ID)) }).
		QueryMenus().Count(ctx)
	if err != nil {
		return true, fmt.Errorf("count admin menu bindings: %w", err)
	}
	return menuCount == 0, nil
}

func initDefaultAdmin(app *Application, ctx context.Context) error {
	adminExists, err := app.DB.User.Query().
		Where(user.IsAdminEQ(true)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("check admin user: %w", err)
	}

	if adminExists {
		adminRole, err := ensureAdminRole(app.DB, ctx)
		if err != nil {
			return err
		}
		log.Println("admin user already exists")
		return initMenu(app.DB, adminRole.ID, ctx)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(app.Env.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	adminRole, err := app.DB.Role.Create().
		SetName("admin").
		SetIsSystem(true).
		SetStatus(constants.StatusActive).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create admin role: %w", err)
	}

	if _, err := app.DB.User.Create().
		SetUsername(app.Env.AdminUsername).
		SetEmail(app.Env.AdminEmail).
		SetPassword(string(hashedPassword)).
		SetStatus(user.StatusActive).
		SetIsAdmin(true).
		AddRoles(adminRole).
		Save(ctx); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	if err := initMenu(app.DB, adminRole.ID, ctx); err != nil {
		return fmt.Errorf("initialize admin menus: %w", err)
	}
	log.Printf("admin user created: %s", app.Env.AdminUsername)
	return nil
}

func ensureAdminRole(client *ent.Client, ctx context.Context) (*ent.Role, error) {
	adminRole, err := client.Role.Query().Where(func(s *sql.Selector) {
		s.Where(sql.EQ("name", "admin"))
	}).First(ctx)
	if ent.IsNotFound(err) {
		adminRole, err = client.Role.Create().
			SetName("admin").
			SetIsSystem(true).
			SetStatus(constants.StatusActive).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("recreate admin role: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("get admin role: %w", err)
	}

	if _, err := client.User.Update().
		Where(user.IsAdminEQ(true)).
		AddRoleIDs(adminRole.ID).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("restore admin role assignment: %w", err)
	}
	return adminRole, nil
}

func initMenu(client *ent.Client, adminRoleID string, ctx context.Context) error {
	count, err := client.Menu.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("check existing menus: %w", err)
	}
	if count == 0 {
		if err := seedDefaultMenus(ctx, client); err != nil {
			return err
		}
		log.Println("menu data initialized")
	}
	return bindMenusToAdmin(client, adminRoleID, ctx)
}

func bindMenusToAdmin(client *ent.Client, adminRoleID string, ctx context.Context) error {
	menus, err := client.Menu.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("fetch menus for admin: %w", err)
	}
	if len(menus) == 0 {
		return nil
	}
	adminRole, err := client.Role.Get(ctx, adminRoleID)
	if err != nil {
		return fmt.Errorf("get admin role: %w", err)
	}
	if _, err := adminRole.Update().AddMenus(menus...).Save(ctx); err != nil {
		return fmt.Errorf("bind menus to admin role: %w", err)
	}
	return nil
}
