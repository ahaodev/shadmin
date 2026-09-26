package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/role"

	_ "github.com/mattn/go-sqlite3"
)

func newAuthorizationStateTestClient(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client, err := ent.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := EnsureAuthorizationState(context.Background(), client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}
	return client
}

func TestWithAuthorizationTxCommitsGenerationWithMutation(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()

	err := WithAuthorizationTx(ctx, client, func(ctx context.Context, tx *ent.Tx) error {
		return tx.Role.Create().SetName("operator").SetStatus("active").Exec(ctx)
	})
	if err != nil {
		t.Fatalf("WithAuthorizationTx: %v", err)
	}

	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 1 {
		t.Fatalf("generation = %d, err = %v; want 1", got, err)
	}
	if exists, err := client.Role.Query().Where(role.Name("operator")).Exist(ctx); err != nil || !exists {
		t.Fatalf("role exists = %v, err = %v; want role committed", exists, err)
	}
}

func TestWithAuthorizationTxRollsBackMutationAndGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	wantErr := errors.New("rollback")

	err := WithAuthorizationTx(ctx, client, func(ctx context.Context, tx *ent.Tx) error {
		if err := tx.Role.Create().SetName("operator").SetStatus("active").Exec(ctx); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 0 {
		t.Fatalf("generation after rollback = %d, err = %v; want 0", got, err)
	}
	if exists, err := client.Role.Query().Where(role.Name("operator")).Exist(ctx); err != nil || exists {
		t.Fatalf("role exists = %v, err = %v after rollback; want absent", exists, err)
	}
}

func TestNestedAuthorizationChangesShareTransactionAndGenerationBump(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()

	err := WithAuthorizationTx(ctx, client, func(ctx context.Context, tx *ent.Tx) error {
		return WithAuthorizationTx(ctx, tx.Client(), func(ctx context.Context, nestedTx *ent.Tx) error {
			return nestedTx.Role.Create().SetName("operator").SetStatus("active").Exec(ctx)
		})
	})
	if err != nil {
		t.Fatalf("nested WithAuthorizationTx: %v", err)
	}

	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 1 {
		t.Fatalf("generation = %d, err = %v; want one bump for the transaction", got, err)
	}
}

func TestUserRepositoryCreateAndRoleReplacementAreAtomic(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()

	roleRepository := NewRoleRepository(client)
	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}
	if err := roleRepository.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	userRepository := NewUserRepository(client)
	user := &domain.User{Username: "alice", Status: domain.UserStatusActive}
	if err := userRepository.CreateWithRoles(ctx, user, []string{role.ID}); err != nil {
		t.Fatalf("create user with role: %v", err)
	}
	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 2 {
		t.Fatalf("generation after create = %d, err = %v; want 2", got, err)
	}

	stored, err := userRepository.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	if len(stored.Roles) != 1 || stored.Roles[0] != role.ID {
		t.Fatalf("created user roles = %v, want [%s]", stored.Roles, role.ID)
	}
	if err := userRepository.UpdateWithRoles(ctx, stored, nil); err != nil {
		t.Fatalf("clear user roles: %v", err)
	}
	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 3 {
		t.Fatalf("generation after role replacement = %d, err = %v; want 3", got, err)
	}
	stored, err = userRepository.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if len(stored.Roles) != 0 {
		t.Fatalf("roles after clearing = %v, want empty", stored.Roles)
	}
}

func TestUserRepositoryOrdinaryUpdateDoesNotBumpAuthorizationGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	userRepository := NewUserRepository(client)
	user := &domain.User{Username: "alice", Status: domain.UserStatusActive}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Nickname = "Alice"
	user.Password = "new-password-hash"
	if err := userRepository.Update(ctx, user); err != nil {
		t.Fatalf("ordinary user update: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)

	user.Status = domain.UserStatusInactive
	if err := userRepository.UpdateWithAuthorization(ctx, user); err != nil {
		t.Fatalf("authorization user update: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 2)
}

func TestUserRepositoryProfileRefreshDoesNotBumpAuthorizationGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	userRepository := NewUserRepository(client)
	user := &domain.User{Username: "alice", Status: domain.UserStatusActive}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := userRepository.UpdateIdentityProfile(ctx, user.ID, "Alice", "avatar"); err != nil {
		t.Fatalf("update identity profile: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)
}

func assertAuthorizationGeneration(t *testing.T, ctx context.Context, client *ent.Client, want int64) {
	t.Helper()
	got, err := CurrentAuthorizationGeneration(ctx, client)
	if err != nil || got != want {
		t.Fatalf("authorization generation = %d, err = %v; want %d", got, err, want)
	}
}

func TestRoleRepositoryMutationsBumpAuthorizationGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	repo := NewRoleRepository(client)
	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}

	if err := repo.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)

	role.Name = "operator-updated"
	role.Status = domain.RoleStatusInactive
	if err := repo.Update(ctx, role); err != nil {
		t.Fatalf("update role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 2)

	if err := repo.Delete(ctx, role.ID); err != nil {
		t.Fatalf("delete role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 3)
}

func TestRoleRepositoryDeleteIfUnusedBumpsOnlyOnCommit(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	roleRepo := NewRoleRepository(client)
	userRepo := NewUserRepository(client)
	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}
	if err := roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)

	user := &domain.User{Username: "alice", Status: domain.UserStatusActive}
	if err := userRepo.CreateWithRoles(ctx, user, []string{role.ID}); err != nil {
		t.Fatalf("create user with role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 2)

	if err := roleRepo.DeleteIfUnused(ctx, role.ID, role.Name); !errors.Is(err, domain.ErrRoleInUse) {
		t.Fatalf("DeleteIfUnused error = %v, want ErrRoleInUse", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 2)

	if err := userRepo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 3)
	if err := roleRepo.DeleteIfUnused(ctx, role.ID, role.Name); err != nil {
		t.Fatalf("delete unused role: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 4)
}

func TestMenuRepositoryMutationsBumpAuthorizationGeneration(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	repo := NewMenuRepository(client)

	root, err := repo.CreateMenu(ctx, &domain.CreateMenuRequest{
		Name: "root", Type: domain.MenuTypeMenu, Visible: "show", Status: domain.MenuStatusActive,
	})
	if err != nil {
		t.Fatalf("create root menu: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 1)

	if _, err := repo.UpdateMenu(ctx, root.ID, &domain.UpdateMenuRequest{
		Name: "root-updated", Type: domain.MenuTypeMenu, Visible: "show", Status: domain.MenuStatusInactive,
	}); err != nil {
		t.Fatalf("update root menu: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 2)

	if err := repo.DeleteMenu(ctx, root.ID); err != nil {
		t.Fatalf("delete root menu: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 3)

	parent, err := repo.CreateMenu(ctx, &domain.CreateMenuRequest{
		Name: "parent", Type: domain.MenuTypeMenu, Visible: "show", Status: domain.MenuStatusActive,
	})
	if err != nil {
		t.Fatalf("create parent menu: %v", err)
	}
	parentID := parent.ID
	if _, err := repo.CreateMenu(ctx, &domain.CreateMenuRequest{
		Name: "child", Type: domain.MenuTypeMenu, Visible: "show", Status: domain.MenuStatusActive, ParentID: &parentID,
	}); err != nil {
		t.Fatalf("create child menu: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 5)

	if err := repo.DeleteMenuTree(ctx, parent.ID); err != nil {
		t.Fatalf("delete menu tree: %v", err)
	}
	assertAuthorizationGeneration(t, ctx, client, 6)
}
