package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"shadmin/domain"
)

func newMenuRequest(name string) *domain.CreateMenuRequest {
	return &domain.CreateMenuRequest{
		Name:    name,
		Type:    domain.MenuTypeMenu,
		Status:  domain.MenuStatusActive,
		Visible: "show",
	}
}

func generation(t *testing.T, repo *entMenuRepository) int64 {
	t.Helper()
	g, err := CurrentAuthorizationGeneration(context.Background(), repo.client)
	if err != nil {
		t.Fatalf("CurrentAuthorizationGeneration: %v", err)
	}
	return g
}

// 菜单的创建 / 更新 / 删除各自与 generation 推进同事务提交，且只推进一次。
func TestMenuWritesBumpAuthorizationGenerationOnce(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	repo := NewMenuRepository(client).(*entMenuRepository)

	created, err := repo.CreateMenu(ctx, newMenuRequest("系统"))
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if got := generation(t, repo); got != 1 {
		t.Fatalf("generation after create = %d, want 1", got)
	}

	if _, err := repo.UpdateMenu(ctx, created.ID, &domain.UpdateMenuRequest{
		Name: "系统管理", Type: domain.MenuTypeMenu, Status: domain.MenuStatusActive, Visible: "show",
	}); err != nil {
		t.Fatalf("UpdateMenu: %v", err)
	}
	if got := generation(t, repo); got != 2 {
		t.Fatalf("generation after update = %d, want 2", got)
	}

	child := newMenuRequest("子菜单")
	child.ParentID = &created.ID
	if _, err := repo.CreateMenu(ctx, child); err != nil {
		t.Fatalf("CreateMenu child: %v", err)
	}
	if err := repo.DeleteMenuTree(ctx, created.ID); err != nil {
		t.Fatalf("DeleteMenuTree: %v", err)
	}
	// 创建子菜单 +1，递归删除父子两个节点合计只 +1。
	if got := generation(t, repo); got != 4 {
		t.Fatalf("generation after tree delete = %d, want 4", got)
	}
	if _, err := repo.GetMenuByID(ctx, created.ID); !errors.Is(err, domain.ErrMenuNotFound) {
		t.Fatalf("GetMenuByID after delete: err = %v, want ErrMenuNotFound", err)
	}
}

// 更新不存在的菜单：返回 ErrMenuNotFound，且 generation 不推进（事务回滚）。
func TestUpdateMissingMenuRollsBackGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	repo := NewMenuRepository(client).(*entMenuRepository)

	_, err := repo.UpdateMenu(context.Background(), "missing", &domain.UpdateMenuRequest{
		Name: "x", Type: domain.MenuTypeMenu, Status: domain.MenuStatusActive, Visible: "show",
	})

	if !errors.Is(err, domain.ErrMenuNotFound) {
		t.Fatalf("err = %v, want ErrMenuNotFound", err)
	}
	if got := generation(t, repo); got != 0 {
		t.Fatalf("generation = %d, want 0 after rollback", got)
	}
}

// 角色删除：仍被用户使用时回滚，不推进 generation；删除成功只推进一次。
func TestDeleteRoleIfUnusedBumpsGenerationOnlyOnSuccess(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	roles := NewRoleRepository(client)

	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}
	if err := roles.Create(ctx, role); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, _ := CurrentAuthorizationGeneration(ctx, client)

	user := client.User.Create().SetUsername("u1").AddRoleIDs(role.ID).SaveX(ctx)
	_ = user
	err := roles.DeleteIfUnused(ctx, role.ID, role.Name)
	if !errors.Is(err, domain.ErrRoleInUse) {
		t.Fatalf("err = %v, want ErrRoleInUse", err)
	}
	if got, _ := CurrentAuthorizationGeneration(ctx, client); got != before {
		t.Fatalf("generation = %d, want unchanged %d after rolled-back delete", got, before)
	}

	client.User.DeleteOneID(user.ID).ExecX(ctx)
	if err := roles.DeleteIfUnused(ctx, role.ID, role.Name); err != nil {
		t.Fatalf("DeleteIfUnused: %v", err)
	}
	if got, _ := CurrentAuthorizationGeneration(ctx, client); got != before+1 {
		t.Fatalf("generation = %d, want %d", got, before+1)
	}
}

// 菜单列表不分页：超过默认页大小也全部返回，并按 keyword 过滤。
func TestGetMenusReturnsAllMatchesWithoutPaging(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	repo := NewMenuRepository(client)

	const n = domain.DefaultPageSize + 5
	for i := 0; i < n; i++ {
		if _, err := repo.CreateMenu(ctx, newMenuRequest(fmt.Sprintf("menu-%02d", i))); err != nil {
			t.Fatalf("CreateMenu: %v", err)
		}
	}
	if _, err := repo.CreateMenu(ctx, newMenuRequest("other")); err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	all, err := repo.GetMenus(ctx, domain.MenuQueryParams{})
	if err != nil {
		t.Fatalf("GetMenus: %v", err)
	}
	if len(all.List) != n+1 || all.Total != n+1 {
		t.Fatalf("list = %d, total = %d; want %d each", len(all.List), all.Total, n+1)
	}

	filtered, err := repo.GetMenus(ctx, domain.MenuQueryParams{Keyword: "other"})
	if err != nil {
		t.Fatalf("GetMenus keyword: %v", err)
	}
	if len(filtered.List) != 1 || filtered.List[0].Name != "other" {
		t.Fatalf("keyword result = %+v, want exactly 'other'", filtered.List)
	}

	empty, err := repo.GetMenus(ctx, domain.MenuQueryParams{Keyword: "no-such"})
	if err != nil {
		t.Fatalf("GetMenus empty: %v", err)
	}
	if empty.List == nil || len(empty.List) != 0 {
		t.Fatalf("empty list = %#v, want non-nil empty slice", empty.List)
	}
}
