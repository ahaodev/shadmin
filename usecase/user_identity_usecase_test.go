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
