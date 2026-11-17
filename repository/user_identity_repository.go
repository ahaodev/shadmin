package repository

import (
	"context"
	"fmt"
	"strings"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/useridentity"
)

type entUserIdentityRepository struct {
	client *ent.Client
}

func NewUserIdentityRepository(client *ent.Client) domain.UserIdentityRepository {
	return &entUserIdentityRepository{client: client}
}

func entUserIdentityToDomain(a *ent.UserIdentity) *domain.UserIdentity {
	if a == nil {
		return nil
	}
	return &domain.UserIdentity{
		ID:              a.ID,
		UserID:          a.UserID,
		Provider:        a.Provider,
		ProviderSubject: a.ProviderSubject,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// FindByProviderAndSubject 未找到时返回 (nil, nil)。
func (r *entUserIdentityRepository) FindByProviderAndSubject(ctx context.Context, provider, subject string) (*domain.UserIdentity, error) {
	if provider == "" || subject == "" {
		return nil, nil
	}

	a, err := clientFromContext(ctx, r.client).UserIdentity.Query().
		Where(
			useridentity.Provider(provider),
			useridentity.ProviderSubject(subject),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query user identity: %w", err)
	}
	return entUserIdentityToDomain(a), nil
}

// Upsert 按 (provider, provider_subject) 写入；已存在时只更新 user_id。
func (r *entUserIdentityRepository) Upsert(ctx context.Context, account *domain.UserIdentity) error {
	if account == nil {
		return fmt.Errorf("user identity is nil")
	}
	if account.Provider == "" || account.ProviderSubject == "" {
		return fmt.Errorf("provider and provider_subject are required")
	}

	existing, err := clientFromContext(ctx, r.client).UserIdentity.Query().
		Where(
			useridentity.Provider(account.Provider),
			useridentity.ProviderSubject(account.ProviderSubject),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("query existing user identity: %w", err)
	}

	if ent.IsNotFound(err) || existing == nil {
		created, createErr := clientFromContext(ctx, r.client).UserIdentity.Create().
			SetUserID(account.UserID).
			SetProvider(account.Provider).
			SetProviderSubject(account.ProviderSubject).
			Save(ctx)
		if createErr != nil {
			if isUniqueConstraintError(createErr) {
				return fmt.Errorf("%w: %w", domain.ErrUserIdentityConflict, createErr)
			}
			return fmt.Errorf("create user identity: %w", createErr)
		}
		account.ID = created.ID
		account.CreatedAt = created.CreatedAt
		account.UpdatedAt = created.UpdatedAt
		return nil
	}

	return r.updateOne(ctx, existing.ID, account)
}

// isUniqueConstraintError 仅识别唯一约束冲突，避免其他约束错误触发登录重试。
func isUniqueConstraintError(err error) bool {
	if !ent.IsConstraintError(err) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicate entry")
}

func (r *entUserIdentityRepository) updateOne(ctx context.Context, id string, account *domain.UserIdentity) error {
	updated, err := clientFromContext(ctx, r.client).UserIdentity.UpdateOneID(id).
		SetUserID(account.UserID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update user identity: %w", err)
	}
	account.ID = updated.ID
	account.CreatedAt = updated.CreatedAt
	account.UpdatedAt = updated.UpdatedAt
	return nil
}
