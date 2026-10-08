package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"shadmin/domain"
)

// fakeUnitOfWork 依次返回 failures 中的错误（不执行 fn）；失败用尽后真正执行 fn。
type fakeUnitOfWork struct {
	failures []error
	calls    int
}

func (f *fakeUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	if len(f.failures) > 0 {
		err := f.failures[0]
		f.failures = f.failures[1:]
		return err
	}
	return fn(ctx)
}

func TestBuildOAuthUsernameUsesLongHashAndFitsSchemaLimit(t *testing.T) {
	username := buildOAuthUsername("github", "subject-1", "abcdefghijklmnopqrstuvwxyz")
	if len(username) != 32 {
		t.Fatalf("username length = %d, want 32", len(username))
	}
	if got := len(usernameSuffix("github", "subject-1")); got != 15 {
		t.Fatalf("username suffix length = %d, want 15", got)
	}
}

func TestResolveOrCreateUserRetriesIdentityConflict(t *testing.T) {
	resolved := &domain.User{ID: "user-1", Status: domain.UserStatusActive}
	uow := &fakeUnitOfWork{failures: []error{
		fmt.Errorf("transaction rolled back: %w", domain.ErrUserIdentityConflict),
	}}
	usecase := &userIdentityUsecase{
		uow:                uow,
		userRepository:     &identityUserRepository{user: resolved},
		identityRepository: &identityRepositoryFake{existing: &domain.UserIdentity{UserID: "user-1"}},
	}

	got, err := usecase.resolveOrCreateUser(context.Background(), "github", domain.UserIdentityProfile{UserID: "subject-1"})
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	if got != resolved {
		t.Fatalf("resolved user = %#v, want %#v", got, resolved)
	}
	if uow.calls != 2 {
		t.Fatalf("transaction attempts = %d, want 2", uow.calls)
	}
}

func TestResolveOrCreateUserDoesNotRetryOtherConstraintErrors(t *testing.T) {
	firstErr := errors.New("foreign key constraint failed")
	uow := &fakeUnitOfWork{failures: []error{firstErr}}
	usecase := &userIdentityUsecase{uow: uow}

	_, err := usecase.resolveOrCreateUser(context.Background(), "github", domain.UserIdentityProfile{UserID: "subject-1"})
	if !errors.Is(err, firstErr) {
		t.Fatalf("resolveOrCreateUser error = %v, want %v", err, firstErr)
	}
	if uow.calls != 1 {
		t.Fatalf("transaction attempts = %d, want 1", uow.calls)
	}
}

type identityRepositoryFake struct {
	domain.UserIdentityRepository
	existing *domain.UserIdentity
	bound    *domain.UserIdentity
}

func (r *identityRepositoryFake) FindByProviderAndSubject(context.Context, string, string) (*domain.UserIdentity, error) {
	return r.existing, nil
}

func (r *identityRepositoryFake) Upsert(_ context.Context, identity *domain.UserIdentity) error {
	r.bound = identity
	return nil
}

type identityUserRepository struct {
	domain.UserRepository
	user    *domain.User
	roleIDs []string
}

func (r *identityUserRepository) CreateWithRoles(_ context.Context, user *domain.User, roleIDs []string) error {
	user.ID = "user-1"
	user.Roles = append([]string(nil), roleIDs...)
	user.IsActive = len(roleIDs) > 0
	r.user = user
	r.roleIDs = append([]string(nil), roleIDs...)
	return nil
}

func (r *identityUserRepository) GetByID(context.Context, string) (*domain.User, error) {
	return r.user, nil
}

func (r *identityUserRepository) UpdateIdentityProfile(_ context.Context, _ string, nickname, avatar string) error {
	r.user.Nickname = nickname
	r.user.Avatar = avatar
	return nil
}

type identityRoleRepository struct {
	domain.RoleRepository
	role    *domain.Role
	lookups int
}

func (r *identityRoleRepository) GetByName(_ context.Context, name string) (*domain.Role, error) {
	r.lookups++
	if name != domain.RoleNameViewer {
		return nil, domain.ErrRoleNotFound
	}
	return r.role, nil
}

func TestNewIdentityUserReceivesViewerRole(t *testing.T) {
	userRepo := &identityUserRepository{}
	roleRepo := &identityRoleRepository{role: &domain.Role{
		ID:     "viewer-role",
		Name:   domain.RoleNameViewer,
		Status: domain.RoleStatusActive,
	}}
	identityRepo := &identityRepositoryFake{}
	usecase := &userIdentityUsecase{
		uow:                &fakeUnitOfWork{},
		userRepository:     userRepo,
		roleRepository:     roleRepo,
		identityRepository: identityRepo,
	}

	user, err := usecase.resolveOrCreateUser(
		context.Background(),
		"github",
		domain.UserIdentityProfile{UserID: "subject-1", Name: "Viewer"},
	)
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	if len(userRepo.roleIDs) != 1 || userRepo.roleIDs[0] != "viewer-role" {
		t.Fatalf("assigned role IDs = %v, want [viewer-role]", userRepo.roleIDs)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "viewer-role" {
		t.Fatalf("returned user roles = %v, want [viewer-role]", user.Roles)
	}
	if identityRepo.bound == nil || identityRepo.bound.UserID != user.ID {
		t.Fatalf("identity binding = %#v, want user ID %q", identityRepo.bound, user.ID)
	}
}

func TestExistingIdentityDoesNotRestoreRemovedViewerRole(t *testing.T) {
	userRepo := &identityUserRepository{user: &domain.User{
		ID:     "user-1",
		Status: domain.UserStatusActive,
	}}
	roleRepo := &identityRoleRepository{role: &domain.Role{
		ID:     "viewer-role",
		Name:   domain.RoleNameViewer,
		Status: domain.RoleStatusActive,
	}}
	usecase := &userIdentityUsecase{
		uow:                &fakeUnitOfWork{},
		userRepository:     userRepo,
		roleRepository:     roleRepo,
		identityRepository: &identityRepositoryFake{existing: &domain.UserIdentity{UserID: "user-1"}},
	}

	user, err := usecase.resolveOrCreateUser(
		context.Background(),
		"github",
		domain.UserIdentityProfile{UserID: "subject-1", Name: "Viewer"},
	)
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	if len(user.Roles) != 0 {
		t.Fatalf("existing user roles = %v, want unchanged empty roles", user.Roles)
	}
	if roleRepo.lookups != 0 {
		t.Fatalf("default role lookups = %d, want none for existing identity", roleRepo.lookups)
	}
	if userRepo.user.Roles != nil {
		t.Fatalf("existing user roles after callback = %v, want nil", userRepo.user.Roles)
	}
}
