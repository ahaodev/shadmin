package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/cacher"
	"shadmin/usecase"
	"sync/atomic"
	"testing"
	"time"
)

type refreshUserRepo struct{ domain.UserRepository }

func (refreshUserRepo) GetByID(context.Context, string) (*domain.User, error) {
	return &domain.User{ID: "alice", Status: domain.UserStatusActive}, nil
}

type refreshIssuer struct {
	domain.TokenIssuer
	claims  *domain.TokenClaims
	ready   chan struct{}
	release chan struct{}
	issued  atomic.Int32
}

func (i *refreshIssuer) ParseRefreshClaims(string) (*domain.TokenClaims, error) { return i.claims, nil }
func (i *refreshIssuer) IssuePair(*domain.User) (*domain.LoginResponse, error) {
	n := i.issued.Add(1)
	if i.ready != nil {
		i.ready <- struct{}{}
		<-i.release
	}
	return &domain.LoginResponse{AccessToken: fmt.Sprint("access-", n), RefreshToken: fmt.Sprint("refresh-", n)}, nil
}

type failingConsumption struct{ domain.TokenBlacklist }

func (failingConsumption) Consume(context.Context, string, time.Time) (bool, error) {
	return false, errors.New("storage unavailable")
}
func TestRefreshConcurrentSingleWinner(t *testing.T) {
	issuer := &refreshIssuer{claims: &domain.TokenClaims{ID: "alice", JTI: "old", ExpiresAt: time.Now().Add(time.Minute)}, ready: make(chan struct{}, 2), release: make(chan struct{})}
	blacklist := auth.NewTokenBlacklist(cacher.NewMemoryCache(cacher.MemoryConfig{}))
	u := usecase.NewLoginUsecase(refreshUserRepo{}, nil, nil, nil, issuer, blacklist, time.Second)
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := u.Refresh(t.Context(), "old"); results <- err }()
	}
	for range 2 {
		select {
		case <-issuer.ready:
		case <-time.After(time.Second):
			t.Fatal("refresh did not reach issuance barrier")
		}
	}
	close(issuer.release)
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrRefreshTokenRevoked) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful rotations = %d", successes)
	}
	if _, err := u.Refresh(t.Context(), "old"); !errors.Is(err, domain.ErrRefreshTokenRevoked) {
		t.Fatalf("replay = %v", err)
	}
}
func TestRefreshFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		claims    *domain.TokenClaims
		blacklist domain.TokenBlacklist
	}{
		{"storage failure", &domain.TokenClaims{ID: "alice", JTI: "old", ExpiresAt: time.Now().Add(time.Minute)}, failingConsumption{}},
		{"missing store", &domain.TokenClaims{ID: "alice", JTI: "old", ExpiresAt: time.Now().Add(time.Minute)}, nil},
		{"expired", &domain.TokenClaims{ID: "alice", JTI: "old", ExpiresAt: time.Now().Add(-time.Second)}, failingConsumption{}},
		{"missing jti", &domain.TokenClaims{ID: "alice", ExpiresAt: time.Now().Add(time.Minute)}, failingConsumption{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := usecase.NewLoginUsecase(refreshUserRepo{}, nil, nil, nil, &refreshIssuer{claims: tc.claims}, tc.blacklist, time.Second)
			if tokens, err := u.Refresh(t.Context(), "old"); err == nil || tokens != nil {
				t.Fatalf("refresh = %v, %v", tokens, err)
			}
		})
	}
}
