package repository

import (
	"context"
	"testing"

	"shadmin/domain"
	entuser "shadmin/ent/user"
)

func TestCreateWithRolesReturnsAndPersistsRoleIDs(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()

	createdRole, err := client.Role.Create().
		SetName(domain.RoleNameViewer).
		SetIsSystem(true).
		SetStatus(domain.RoleStatusActive).
		Save(ctx)
	if err != nil {
		t.Fatalf("create viewer role: %v", err)
	}

	user := &domain.User{
		Username: "oauth-user",
		Status:   domain.UserStatusActive,
	}
	if err := NewUserRepository(client).CreateWithRoles(ctx, user, []string{createdRole.ID}); err != nil {
		t.Fatalf("create user with viewer role: %v", err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != createdRole.ID {
		t.Fatalf("returned role IDs = %v, want [%s]", user.Roles, createdRole.ID)
	}
	if !user.IsActive {
		t.Fatal("user with a role should be marked active")
	}

	persisted, err := client.User.Query().Where(entuser.ID(user.ID)).First(ctx)
	if err != nil {
		t.Fatalf("query created user: %v", err)
	}
	roleIDs, err := persisted.QueryRoles().IDs(ctx)
	if err != nil {
		t.Fatalf("query persisted user roles: %v", err)
	}
	if len(roleIDs) != 1 || roleIDs[0] != createdRole.ID {
		t.Fatalf("persisted role IDs = %v, want [%s]", roleIDs, createdRole.ID)
	}
}
