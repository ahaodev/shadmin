package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"shadmin/domain"
)

type retryBindingRepository struct {
	domain.UserIdentityRepository
	attempts int
	firstErr error
	resolved *domain.User
}

func (r *retryBindingRepository) WithUserBindingTx(context.Context, domain.UserIdentityBindingTxFunc) (*domain.User, error) {
	r.attempts++
	if r.attempts == 1 {
		return nil, r.firstErr
	}
	return r.resolved, nil
}

func TestResolveOrCreateUserRetriesIdentityConflict(t *testing.T) {
	resolved := &domain.User{ID: "user-1", Status: domain.UserStatusActive}
	repo := &retryBindingRepository{
		firstErr: fmt.Errorf("transaction rolled back: %w", domain.ErrUserIdentityConflict),
		resolved: resolved,
	}
	usecase := &userIdentityUsecase{identityRepository: repo}

	got, err := usecase.resolveOrCreateUser(context.Background(), "github", domain.UserIdentityProfile{UserID: "subject-1"})
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	if got != resolved {
		t.Fatalf("resolved user = %#v, want %#v", got, resolved)
	}
	if repo.attempts != 2 {
		t.Fatalf("transaction attempts = %d, want 2", repo.attempts)
	}
}

func TestResolveOrCreateUserDoesNotRetryOtherConstraintErrors(t *testing.T) {
	firstErr := errors.New("foreign key constraint failed")
	repo := &retryBindingRepository{firstErr: firstErr}
	usecase := &userIdentityUsecase{identityRepository: repo}

	_, err := usecase.resolveOrCreateUser(context.Background(), "github", domain.UserIdentityProfile{UserID: "subject-1"})
	if !errors.Is(err, firstErr) {
		t.Fatalf("resolveOrCreateUser error = %v, want %v", err, firstErr)
	}
	if repo.attempts != 1 {
		t.Fatalf("transaction attempts = %d, want 1", repo.attempts)
	}
}

type newIdentityBindingRepository struct {
	domain.UserIdentityRepository
	userRepo domain.UserRepository
	roleRepo domain.RoleRepository
	bound    *domain.UserIdentity
	existing *domain.UserIdentity
}

func (r *newIdentityBindingRepository) WithUserBindingTx(ctx context.Context, fn domain.UserIdentityBindingTxFunc) (*domain.User, error) {
	return fn(ctx, r.userRepo, r, r.roleRepo)
}

func (r *newIdentityBindingRepository) FindByProviderAndSubject(context.Context, string, string) (*domain.UserIdentity, error) {
	return r.existing, nil
}

func (r *newIdentityBindingRepository) Upsert(_ context.Context, identity *domain.UserIdentity) error {
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
	identityRepo := &newIdentityBindingRepository{
		userRepo: userRepo,
		roleRepo: roleRepo,
	}
	usecase := &userIdentityUsecase{identityRepository: identityRepo}

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
	identityRepo := &newIdentityBindingRepository{
		userRepo: userRepo,
		roleRepo: roleRepo,
		existing: &domain.UserIdentity{UserID: "user-1"},
	}
	usecase := &userIdentityUsecase{identityRepository: identityRepo}

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
