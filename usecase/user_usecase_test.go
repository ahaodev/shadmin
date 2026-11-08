package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"shadmin/domain"

	"golang.org/x/crypto/bcrypt"
)

type userUpdateTestRepository struct {
	domain.UserRepository
	user                         *domain.User
	updateCalls                  int
	updateWithAuthorizationCalls int
	updateWithRolesCalls         int
	updatedRoleIDs               []string
}

func TestUpdateUserPasswordRejectsInvalidNewPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"", "short", strings.Repeat("x", 73)} {
		repo := &userUpdateTestRepository{user: &domain.User{ID: "user-1", Password: string(hash)}}
		uc := NewUserUsecase(repo, time.Second)
		err := uc.UpdateUserPassword(context.Background(), "user-1", domain.PasswordUpdate{CurrentPassword: "old-password", NewPassword: password})
		if !errors.Is(err, domain.ErrPasswordLength) || repo.updateCalls != 0 {
			t.Fatalf("UpdateUserPassword = %v, writes = %d; want rejected without write", err, repo.updateCalls)
		}
	}
}

func (r *userUpdateTestRepository) GetByID(context.Context, string) (*domain.User, error) {
	user := *r.user
	return &user, nil
}

func (r *userUpdateTestRepository) Update(context.Context, *domain.User) error {
	r.updateCalls++
	return nil
}

func (r *userUpdateTestRepository) UpdateWithAuthorization(context.Context, *domain.User) error {
	r.updateWithAuthorizationCalls++
	return nil
}

func (r *userUpdateTestRepository) UpdateWithRoles(_ context.Context, _ *domain.User, roleIDs []string) error {
	r.updateWithRolesCalls++
	r.updatedRoleIDs = roleIDs
	return nil
}

func TestUpdateUserPartial_EmptyRoleIDsClearsRoles(t *testing.T) {
	repo := &userUpdateTestRepository{user: &domain.User{ID: "user-1", Status: domain.UserStatusActive}}
	useCase := NewUserUsecase(repo, time.Second)

	if err := useCase.UpdateUserPartial(context.Background(), "user-1", domain.UserUpdateRequest{RoleIDs: []string{}}); err != nil {
		t.Fatalf("UpdateUserPartial: %v", err)
	}
	if repo.updateWithRolesCalls != 1 || repo.updateCalls != 0 {
		t.Fatalf("Update calls = %d, UpdateWithRoles calls = %d; want 0 and 1", repo.updateCalls, repo.updateWithRolesCalls)
	}
	if repo.updatedRoleIDs == nil || len(repo.updatedRoleIDs) != 0 {
		t.Fatalf("role IDs passed to repository = %#v, want explicit empty slice", repo.updatedRoleIDs)
	}
}

func TestUpdateUserPartial_StatusUsesAuthorizationUpdate(t *testing.T) {
	repo := &userUpdateTestRepository{user: &domain.User{ID: "user-1", Status: domain.UserStatusActive}}
	useCase := NewUserUsecase(repo, time.Second)
	inactive := domain.UserStatusInactive

	if err := useCase.UpdateUserPartial(context.Background(), "user-1", domain.UserUpdateRequest{Status: &inactive}); err != nil {
		t.Fatalf("UpdateUserPartial: %v", err)
	}
	if repo.updateWithAuthorizationCalls != 1 || repo.updateCalls != 0 || repo.updateWithRolesCalls != 0 {
		t.Fatalf("Update calls = %d, UpdateWithAuthorization calls = %d, UpdateWithRoles calls = %d; want 0, 1, 0", repo.updateCalls, repo.updateWithAuthorizationCalls, repo.updateWithRolesCalls)
	}
}

func TestUpdateUserPartial_OmittedRoleIDsLeaveRolesUnchanged(t *testing.T) {
	repo := &userUpdateTestRepository{user: &domain.User{ID: "user-1", Status: domain.UserStatusActive}}
	useCase := NewUserUsecase(repo, time.Second)

	if err := useCase.UpdateUserPartial(context.Background(), "user-1", domain.UserUpdateRequest{}); err != nil {
		t.Fatalf("UpdateUserPartial: %v", err)
	}
	if repo.updateCalls != 1 || repo.updateWithRolesCalls != 0 {
		t.Fatalf("Update calls = %d, UpdateWithRoles calls = %d; want 1 and 0", repo.updateCalls, repo.updateWithRolesCalls)
	}
}
