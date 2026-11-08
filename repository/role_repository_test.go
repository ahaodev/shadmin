package repository

import (
	"context"
	"errors"
	"testing"

	"shadmin/domain"
	"shadmin/ent"
)

// 角色改名撞上已存在的名称时，唯一索引冲突必须映射为 ErrRoleNameExists（→409），
// 而不是作为原始数据库错误冒泡成 500。
func TestRoleRepositoryUpdateMapsDuplicateName(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	repo := NewRoleRepository(client)
	ctx := context.Background()

	for _, name := range []string{"operator", "auditor"} {
		if err := repo.Create(ctx, &domain.Role{Name: name, Status: domain.RoleStatusActive}); err != nil {
			t.Fatalf("create role %s: %v", name, err)
		}
	}

	auditor, err := repo.GetByName(ctx, "auditor")
	if err != nil {
		t.Fatalf("get auditor: %v", err)
	}

	auditor.Name = "operator"
	if err := repo.Update(ctx, auditor); !errors.Is(err, domain.ErrRoleNameExists) {
		t.Fatalf("update to duplicate name = %v; want ErrRoleNameExists", err)
	}
}

func TestRoleRepositoryUpdatePreservesMenuConstraintError(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	repo := NewRoleRepository(client)
	ctx := context.Background()
	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}
	if err := repo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}

	role.MenuIDs = []string{"missing-menu"}
	err := repo.Update(ctx, role)
	if !ent.IsConstraintError(err) || errors.Is(err, domain.ErrRoleNameExists) {
		t.Fatalf("missing menu error = %v; want original constraint error", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)
	stored, err := repo.GetByID(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.MenuIDs) != 0 {
		t.Fatalf("failed update persisted menus: %v", stored.MenuIDs)
	}
}
