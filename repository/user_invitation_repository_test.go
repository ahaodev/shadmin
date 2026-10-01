package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"shadmin/domain"
	"shadmin/internal/constants"
)

func TestUserInvitationRepositoryCreatesRotatesAndConsumesInvite(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	roleRepository := NewRoleRepository(client)
	invitationRepository := NewUserInvitationRepository(client)
	userRepository := NewUserRepository(client)
	role := &domain.Role{Name: "operator", Status: domain.RoleStatusActive}
	if err := roleRepository.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	now := time.Now()
	invitedAt := now
	first := &domain.User{
		Username:  "invite_first",
		Email:     "invitee@example.com",
		Source:    constants.UserSourceLocal,
		Status:    constants.UserStatusInvited,
		InvitedAt: &invitedAt,
		InvitedBy: "admin-1",
	}
	created, err := invitationRepository.CreateOrRotate(ctx, first, &domain.UserInvitation{
		TokenHash: "first-token-hash",
		ExpiresAt: now.Add(time.Hour),
		CreatedBy: "admin-1",
		CreatedAt: now,
	}, []string{role.ID})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	stored, err := userRepository.GetByIdentifier(ctx, "invitee@example.com")
	if err != nil {
		t.Fatalf("get invited user: %v", err)
	}
	if stored.Status != constants.UserStatusInvited || stored.Password != "" || len(stored.Roles) != 1 {
		t.Fatalf("invited user state = %#v; want invited, no password, one role", stored)
	}

	updatedRole := &domain.Role{Name: "reader", Status: domain.RoleStatusActive}
	if err := roleRepository.Create(ctx, updatedRole); err != nil {
		t.Fatalf("create updated role: %v", err)
	}
	second := &domain.User{
		Username:  "invite_second",
		Email:     "invitee@example.com",
		Source:    constants.UserSourceLocal,
		Status:    constants.UserStatusInvited,
		InvitedAt: &invitedAt,
		InvitedBy: "admin-2",
	}
	rotated, err := invitationRepository.CreateOrRotate(ctx, second, &domain.UserInvitation{
		TokenHash: "second-token-hash",
		ExpiresAt: now.Add(time.Hour),
		CreatedBy: "admin-2",
		CreatedAt: now.Add(time.Minute),
	}, []string{updatedRole.ID})
	if err != nil {
		t.Fatalf("rotate invitation: %v", err)
	}
	if rotated.ID != created.ID {
		t.Fatalf("rotated user ID = %q, want original %q", rotated.ID, created.ID)
	}

	invitations, err := client.UserInvitation.Query().All(ctx)
	if err != nil {
		t.Fatalf("list invitation records: %v", err)
	}
	if len(invitations) != 2 || invitations[0].RevokedAt.IsZero() == invitations[1].RevokedAt.IsZero() {
		t.Fatalf("invitation records = %#v; want one revoked and one active", invitations)
	}
	stored, err = userRepository.GetByIdentifier(ctx, "invitee@example.com")
	if err != nil {
		t.Fatalf("get rotated invited user: %v", err)
	}
	if stored.ID != created.ID {
		t.Fatalf("stored user ID = %q, want original %q", stored.ID, created.ID)
	}
	if len(stored.Roles) != 1 || stored.Roles[0] != updatedRole.ID {
		t.Fatalf("roles after rotation = %v, want [%s]", stored.Roles, updatedRole.ID)
	}

	if err := invitationRepository.Accept(ctx, "first-token-hash", "invitee", "hashed-password", now); !errors.Is(err, domain.ErrInvalidInvitation) {
		t.Fatalf("accept revoked invitation error = %v, want ErrInvalidInvitation", err)
	}
	if err := invitationRepository.Accept(ctx, "second-token-hash", "invitee", "hashed-password", now); err != nil {
		t.Fatalf("accept current invitation: %v", err)
	}
	if err := invitationRepository.Accept(ctx, "second-token-hash", "invitee", "hashed-password", now); !errors.Is(err, domain.ErrInvalidInvitation) {
		t.Fatalf("accept used invitation error = %v, want ErrInvalidInvitation", err)
	}

	stored, err = userRepository.GetByIdentifier(ctx, "invitee@example.com")
	if err != nil {
		t.Fatalf("get activated user: %v", err)
	}
	if stored.Username != "invitee" || stored.Status != constants.UserStatusActive || stored.Password != "hashed-password" {
		t.Fatalf("activated user = %#v; want active user with accepted credentials", stored)
	}
	if strings.Contains(stored.Password, "first-token-hash") {
		t.Fatal("invitation token hash was stored as the user password")
	}
}
