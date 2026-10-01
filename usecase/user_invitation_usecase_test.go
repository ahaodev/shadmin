package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"shadmin/domain"
	"shadmin/internal/constants"

	"golang.org/x/crypto/bcrypt"
)

type invitationRepositoryStub struct {
	domain.UserInvitationRepository
	user       *domain.User
	invitation *domain.UserInvitation
	roleIDs    []string
	tokenHash  string
	username   string
	password   string
}

func (r *invitationRepositoryStub) CreateOrRotate(_ context.Context, user *domain.User, invitation *domain.UserInvitation, roleIDs []string) (*domain.User, error) {
	r.user = user
	r.invitation = invitation
	r.roleIDs = append([]string(nil), roleIDs...)
	return &domain.User{ID: "user-1", Email: user.Email, Status: user.Status}, nil
}

func (r *invitationRepositoryStub) Accept(_ context.Context, tokenHash, username, passwordHash string, _ time.Time) error {
	r.tokenHash = tokenHash
	r.username = username
	r.password = passwordHash
	return nil
}

type invitationRoleRepositoryStub struct {
	domain.RoleRepository
	roles []*domain.Role
}

func (r *invitationRoleRepositoryStub) GetByIDs(context.Context, []string) ([]*domain.Role, error) {
	return r.roles, nil
}

type invitationMailerStub struct {
	domain.InvitationMailer
	to   string
	url  string
	text string
	err  error
}

func (m *invitationMailerStub) SendInvitation(_ context.Context, to, invitationURL, message string) error {
	m.to = to
	m.url = invitationURL
	m.text = message
	return m.err
}

func TestUserInvitationUsecaseInviteStoresOnlyTokenHashAndSendsLink(t *testing.T) {
	repository := &invitationRepositoryStub{}
	mailer := &invitationMailerStub{}
	roles := &invitationRoleRepositoryStub{roles: []*domain.Role{{ID: "role-1", Name: "operator", Status: constants.StatusActive}}}
	usecase := NewUserInvitationUsecase(repository, roles, mailer, "https://shadmin.example/accept-invitation", 72*time.Hour, time.Second)

	user, err := usecase.Invite(context.Background(), &domain.InviteUserRequest{
		Email:   "  Person@Example.com ",
		RoleIDs: []string{"role-1", "role-1"},
		Message: "Welcome!",
	}, "admin-1")
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if user.ID != "user-1" || repository.user.Status != constants.UserStatusInvited {
		t.Fatalf("invited user = %#v, stored user = %#v", user, repository.user)
	}
	if repository.user.Email != "person@example.com" || repository.user.Password != "" {
		t.Fatalf("invitation account email/password = %q/%q", repository.user.Email, repository.user.Password)
	}
	if len(repository.roleIDs) != 1 || repository.roleIDs[0] != "role-1" {
		t.Fatalf("role IDs = %v, want one deduplicated role", repository.roleIDs)
	}
	if mailer.to != "person@example.com" || mailer.text != "Welcome!" {
		t.Fatalf("mailer recipient/message = %q/%q", mailer.to, mailer.text)
	}

	fragment := strings.TrimPrefix(mailer.url, "https://shadmin.example/accept-invitation#token=")
	if fragment == mailer.url || fragment == "" {
		t.Fatalf("invitation URL has no fragment token: %q", mailer.url)
	}
	token, err := url.QueryUnescape(fragment)
	if err != nil {
		t.Fatalf("decode invitation token: %v", err)
	}
	sum := sha256.Sum256([]byte(token))
	if repository.invitation.TokenHash != hex.EncodeToString(sum[:]) {
		t.Fatal("repository did not receive the hash of the emailed token")
	}
	if repository.invitation.ExpiresAt.Sub(repository.invitation.CreatedAt) != 72*time.Hour {
		t.Fatalf("invitation lifetime = %s, want 72h", repository.invitation.ExpiresAt.Sub(repository.invitation.CreatedAt))
	}
}

func TestUserInvitationUsecaseAcceptHashesPasswordAndToken(t *testing.T) {
	repository := &invitationRepositoryStub{}
	usecase := NewUserInvitationUsecase(repository, &invitationRoleRepositoryStub{}, &invitationMailerStub{}, "https://shadmin.example/accept-invitation", time.Hour, time.Second)

	if err := usecase.Accept(context.Background(), &domain.AcceptInvitationRequest{
		Token:    "raw-invitation-token",
		Username: " invitee ",
		Password: "password-123",
	}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if repository.tokenHash != hashInvitationToken("raw-invitation-token") {
		t.Fatal("repository received raw token instead of token hash")
	}
	if repository.username != "invitee" {
		t.Fatalf("username = %q, want trimmed username", repository.username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repository.password), []byte("password-123")); err != nil {
		t.Fatalf("stored password is not a bcrypt hash of the supplied password: %v", err)
	}
}

func TestUserInvitationUsecaseRejectsUnassignableRoles(t *testing.T) {
	repository := &invitationRepositoryStub{}
	roles := &invitationRoleRepositoryStub{roles: []*domain.Role{{ID: "admin", Name: "admin", IsSystem: true, Status: constants.StatusActive}}}
	mailer := &invitationMailerStub{}
	usecase := NewUserInvitationUsecase(repository, roles, mailer, "https://shadmin.example/accept-invitation", time.Hour, time.Second)

	_, err := usecase.Invite(context.Background(), &domain.InviteUserRequest{Email: "user@example.com", RoleIDs: []string{"admin"}}, "admin-1")
	if !errors.Is(err, domain.ErrInvitationRoleInvalid) {
		t.Fatalf("Invite error = %v, want ErrInvitationRoleInvalid", err)
	}
	if repository.invitation != nil || mailer.url != "" {
		t.Fatal("invitation was persisted or emailed with an unassignable role")
	}
}
