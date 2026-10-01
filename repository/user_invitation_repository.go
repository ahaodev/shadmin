package repository

import (
	"context"
	"fmt"
	"time"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/role"
	"shadmin/ent/user"
	"shadmin/ent/userinvitation"
	"shadmin/internal/constants"
)

type entUserInvitationRepository struct {
	client *ent.Client
}

func NewUserInvitationRepository(client *ent.Client) domain.UserInvitationRepository {
	return &entUserInvitationRepository{client: client}
}

func (r *entUserInvitationRepository) CreateOrRotate(ctx context.Context, account *domain.User, invitation *domain.UserInvitation, roleIDs []string) (*domain.User, error) {
	if account == nil || invitation == nil || account.InvitedAt == nil {
		return nil, fmt.Errorf("user, invited_at, and invitation are required")
	}

	var savedUser *domain.User
	err := WithAuthorizationTx(ctx, r.client, func(txCtx context.Context, tx *ent.Tx) error {
		client := tx.Client()
		activeRoles, err := client.Role.Query().
			Where(role.IDIn(roleIDs...), role.Status(constants.StatusActive)).
			All(txCtx)
		if err != nil {
			return fmt.Errorf("validate invitation roles: %w", err)
		}
		if len(roleIDs) == 0 || len(activeRoles) != len(roleIDs) {
			return domain.ErrInvitationRoleInvalid
		}

		existing, err := client.User.Query().
			Where(
				user.EmailEqualFold(account.Email),
				user.SourceEQ(user.Source(constants.UserSourceLocal)),
			).
			Only(txCtx)

		var stored *ent.User
		switch {
		case err == nil:
			if string(existing.Status) != constants.UserStatusInvited || existing.IsAdmin {
				return domain.ErrUserAlreadyExists
			}

			update := client.User.UpdateOneID(existing.ID).
				SetStatus(user.Status(constants.UserStatusInvited)).
				SetInvitedAt(*account.InvitedAt).
				SetInvitedBy(account.InvitedBy).
				ClearRoles().
				AddRoleIDs(roleIDs...)
			stored, err = update.Save(txCtx)
			if err != nil {
				return fmt.Errorf("update pending invited user: %w", err)
			}
			if _, err := client.UserInvitation.Update().
				Where(
					userinvitation.HasUserWith(user.ID(existing.ID)),
					userinvitation.AcceptedAtIsNil(),
					userinvitation.RevokedAtIsNil(),
				).
				SetRevokedAt(invitation.CreatedAt).
				Save(txCtx); err != nil {
				return fmt.Errorf("revoke previous user invitations: %w", err)
			}
		case ent.IsNotFound(err):
			create := client.User.Create().
				SetUsername(account.Username).
				SetEmail(account.Email).
				SetSource(user.Source(constants.UserSourceLocal)).
				SetStatus(user.Status(constants.UserStatusInvited)).
				SetInvitedAt(*account.InvitedAt).
				SetInvitedBy(account.InvitedBy).
				AddRoleIDs(roleIDs...)
			stored, err = create.Save(txCtx)
			if err != nil {
				if isUniqueConstraintError(err) {
					return domain.ErrUserAlreadyExists
				}
				return fmt.Errorf("create invited user: %w", err)
			}
		default:
			return fmt.Errorf("find existing invited user: %w", err)
		}

		invitation.UserID = stored.ID
		_, err = client.UserInvitation.Create().
			SetUserID(stored.ID).
			SetTokenHash(invitation.TokenHash).
			SetExpiresAt(invitation.ExpiresAt).
			SetCreatedBy(invitation.CreatedBy).
			SetCreatedAt(invitation.CreatedAt).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("create invitation record: %w", err)
		}

		savedUser = &domain.User{
			ID:        stored.ID,
			Username:  stored.Username,
			Email:     account.Email,
			Source:    string(stored.Source),
			Status:    string(stored.Status),
			Roles:     append([]string(nil), roleIDs...),
			InvitedAt: account.InvitedAt,
			InvitedBy: account.InvitedBy,
			CreatedAt: stored.CreatedAt,
			UpdatedAt: stored.UpdatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create user invitation: %w", err)
	}
	return savedUser, nil
}

func (r *entUserInvitationRepository) Accept(ctx context.Context, tokenHash, username, passwordHash string, now time.Time) error {
	return WithAuthorizationTx(ctx, r.client, func(txCtx context.Context, tx *ent.Tx) error {
		invitation, err := tx.UserInvitation.Query().
			Where(userinvitation.TokenHash(tokenHash)).
			WithUser().
			Only(txCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrInvalidInvitation
			}
			return fmt.Errorf("find invitation: %w", err)
		}
		if !invitation.AcceptedAt.IsZero() || !invitation.RevokedAt.IsZero() || !now.Before(invitation.ExpiresAt) {
			return domain.ErrInvalidInvitation
		}

		invitedUser, err := invitation.Edges.UserOrErr()
		if err != nil {
			return fmt.Errorf("load invitation user: %w", err)
		}
		if invitedUser.Status != user.Status(constants.UserStatusInvited) {
			return domain.ErrInvalidInvitation
		}

		if _, err := tx.UserInvitation.UpdateOneID(invitation.ID).
			SetAcceptedAt(now).
			Save(txCtx); err != nil {
			return fmt.Errorf("consume invitation: %w", err)
		}
		if _, err := tx.User.UpdateOneID(invitedUser.ID).
			SetUsername(username).
			SetPassword(passwordHash).
			SetStatus(user.Status(constants.UserStatusActive)).
			Save(txCtx); err != nil {
			if isUniqueConstraintError(err) {
				return domain.ErrUsernameExists
			}
			return fmt.Errorf("activate invited user: %w", err)
		}
		return nil
	})
}

var _ domain.UserInvitationRepository = (*entUserInvitationRepository)(nil)
