package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/rs/xid"
	"golang.org/x/crypto/bcrypt"

	"shadmin/domain"
	"shadmin/internal/constants"
	"shadmin/pkg"
)

type userInvitationUsecase struct {
	invitationRepository domain.UserInvitationRepository
	roleRepository       domain.RoleRepository
	mailer               domain.InvitationMailer
	acceptURL            string
	expiry               time.Duration
	contextTimeout       time.Duration
}

func NewUserInvitationUsecase(
	invitationRepository domain.UserInvitationRepository,
	roleRepository domain.RoleRepository,
	mailer domain.InvitationMailer,
	acceptURL string,
	expiry time.Duration,
	timeout time.Duration,
) domain.UserInvitationUseCase {
	if expiry <= 0 {
		expiry = 72 * time.Hour
	}
	return &userInvitationUsecase{
		invitationRepository: invitationRepository,
		roleRepository:       roleRepository,
		mailer:               mailer,
		acceptURL:            strings.TrimSpace(acceptURL),
		expiry:               expiry,
		contextTimeout:       timeout,
	}
}

func (u *userInvitationUsecase) Invite(ctx context.Context, request *domain.InviteUserRequest, invitedBy string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	if u.mailer == nil {
		return nil, domain.ErrInvitationEmailNotConfigured
	}
	if request == nil {
		return nil, fmt.Errorf("invitation request is required")
	}
	if strings.TrimSpace(invitedBy) == "" {
		return nil, fmt.Errorf("inviter is required")
	}
	if u.acceptURL == "" {
		return nil, fmt.Errorf("invitation acceptance URL is not configured")
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))
	roleIDs, err := u.validateRoleIDs(ctx, request.RoleIDs)
	if err != nil {
		return nil, err
	}

	token, err := newInvitationToken()
	if err != nil {
		return nil, fmt.Errorf("generate invitation token: %w", err)
	}
	now := time.Now()
	invitedAt := now
	user := &domain.User{
		Username:  "invite_" + xid.New().String(),
		Email:     email,
		Source:    constants.UserSourceLocal,
		Status:    constants.UserStatusInvited,
		InvitedAt: &invitedAt,
		InvitedBy: invitedBy,
	}
	invitation := &domain.UserInvitation{
		TokenHash: hashInvitationToken(token),
		ExpiresAt: now.Add(u.expiry),
		CreatedBy: invitedBy,
		CreatedAt: now,
	}

	createdUser, err := u.invitationRepository.CreateOrRotate(ctx, user, invitation, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	link := strings.TrimRight(u.acceptURL, "/") + "#token=" + token
	if err := u.mailer.SendInvitation(ctx, email, link, request.Message); err != nil {
		safeError := strings.ReplaceAll(err.Error(), email, "[redacted email]")
		safeError = strings.ReplaceAll(safeError, link, "[redacted invitation URL]")
		safeError = strings.ReplaceAll(safeError, token, "[redacted invitation token]")
		pkg.Log.Errorf("Invitation email delivery failed for user %s: %s", createdUser.ID, safeError)
		return createdUser, fmt.Errorf("%w: %w", domain.ErrInvitationDeliveryFailed, err)
	}
	return createdUser, nil
}

func (u *userInvitationUsecase) Accept(ctx context.Context, request *domain.AcceptInvitationRequest) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	if request == nil {
		return domain.ErrInvalidInvitation
	}
	token := strings.TrimSpace(request.Token)
	username := strings.TrimSpace(request.Username)
	passwordLength := len([]byte(request.Password))
	if token == "" || username == "" {
		return domain.ErrInvalidInvitation
	}
	if passwordLength < 8 || passwordLength > 72 {
		return domain.ErrInvitationPasswordInvalid
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash invited user password: %w", err)
	}
	if err := u.invitationRepository.Accept(ctx, hashInvitationToken(token), username, string(passwordHash), time.Now()); err != nil {
		return fmt.Errorf("accept user invitation: %w", err)
	}
	return nil
}

func (u *userInvitationUsecase) validateRoleIDs(ctx context.Context, requested []string) ([]string, error) {
	unique := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, domain.ErrInvitationRoleInvalid
	}

	roles, err := u.roleRepository.GetByIDs(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("get invitation roles: %w", err)
	}
	if len(roles) != len(unique) {
		return nil, domain.ErrInvitationRoleInvalid
	}
	for _, role := range roles {
		if role.Status != constants.StatusActive || (role.IsSystem && role.Name != domain.RoleNameViewer) {
			return nil, domain.ErrInvitationRoleInvalid
		}
	}
	return unique, nil
}

func newInvitationToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var _ domain.UserInvitationUseCase = (*userInvitationUsecase)(nil)
