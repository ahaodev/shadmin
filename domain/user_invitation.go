package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserAlreadyExists            = errors.New("user with this email already exists")
	ErrInvalidInvitation            = errors.New("invitation is invalid or expired")
	ErrInvitationRoleInvalid        = errors.New("one or more invitation roles are unavailable")
	ErrUsernameExists               = errors.New("username already exists")
	ErrInvitationEmailNotConfigured = errors.New("invitation email is not configured")
	ErrInvitationDeliveryFailed     = errors.New("invitation was created but email delivery failed")
	ErrInvitationPasswordInvalid    = errors.New("invitation password must be 8 to 72 bytes")
)

type AcceptInvitationRequest struct {
	Token    string `json:"token" binding:"required"`
	Username string `json:"username" binding:"required,max=32"`
	Password string `json:"password" binding:"required,min=8"`
}

type UserInvitation struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedBy string
	CreatedAt time.Time
}

type UserInvitationRepository interface {
	CreateOrRotate(ctx context.Context, user *User, invitation *UserInvitation, roleIDs []string) (*User, error)
	Accept(ctx context.Context, tokenHash, username, passwordHash string, now time.Time) error
}

type InvitationMailer interface {
	SendInvitation(ctx context.Context, to, invitationURL, message string) error
}

type UserInvitationUseCase interface {
	Invite(ctx context.Context, request *InviteUserRequest, invitedBy string) (*User, error)
	Accept(ctx context.Context, request *AcceptInvitationRequest) error
}
