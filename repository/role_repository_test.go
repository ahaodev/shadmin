package repository

import (
	"context"
	"errors"
	"testing"

	"shadmin/domain"
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
