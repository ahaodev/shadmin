package repository

import (
	"context"
	"errors"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/useridentity"
)

type entUserIdentityRepository struct {
	client *ent.Client
}

// NewUserIdentityRepository 构造第三方账号绑定的 ent 仓储实现
func NewUserIdentityRepository(client *ent.Client) domain.UserIdentityRepository {
	return &entUserIdentityRepository{client: client}
}

// entUserIdentityToDomain 把 ent.UserIdentity 转成 domain.UserIdentity
func entUserIdentityToDomain(a *ent.UserIdentity) *domain.UserIdentity {
	if a == nil {
		return nil
	}
	return &domain.UserIdentity{
		ID:              a.ID,
		UserID:          a.UserID,
		Provider:        a.Provider,
		ProviderSubject: a.ProviderSubject,
		Email:           a.Email,
		Name:            a.Name,
		AvatarURL:       a.AvatarURL,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// FindByProviderAndSubject 通过 provider + 第三方用户ID 查找绑定。
// 未找到时返回 (nil, nil)，调用方据此决定是否走"按邮箱匹配/创建新用户"分支。
func (r *entUserIdentityRepository) FindByProviderAndSubject(ctx context.Context, provider, subject string) (*domain.UserIdentity, error) {
	if provider == "" || subject == "" {
		return nil, nil
	}

	a, err := r.client.UserIdentity.Query().
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

// FindByUserID 查询某用户绑定的全部第三方账号
func (r *entUserIdentityRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.UserIdentity, error) {
	if userID == "" {
		return nil, nil
	}

	records, err := r.client.UserIdentity.Query().
		Where(useridentity.UserID(userID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query user identities by user id: %w", err)
	}

	result := make([]*domain.UserIdentity, 0, len(records))
	for _, a := range records {
		result = append(result, entUserIdentityToDomain(a))
	}
	return result, nil
}

// Upsert 按 (provider, provider_subject) 创建或更新绑定。
// 绑定存在时更新 user_id/email/name/avatar_url，不存在时新建。
func (r *entUserIdentityRepository) Upsert(ctx context.Context, account *domain.UserIdentity) error {
	if account == nil {
		return fmt.Errorf("user identity is nil")
	}
	if account.Provider == "" || account.ProviderSubject == "" {
		return fmt.Errorf("provider and provider_subject are required")
	}

	existing, err := r.client.UserIdentity.Query().
		Where(
			useridentity.Provider(account.Provider),
			useridentity.ProviderSubject(account.ProviderSubject),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("query existing user identity: %w", err)
	}

	if ent.IsNotFound(err) || existing == nil {
		created, createErr := r.client.UserIdentity.Create().
			SetUserID(account.UserID).
			SetProvider(account.Provider).
			SetProviderSubject(account.ProviderSubject).
			SetEmail(account.Email).
			SetName(account.Name).
			SetAvatarURL(account.AvatarURL).
			Save(ctx)
		if createErr != nil {
			if ent.IsConstraintError(createErr) {
				return fmt.Errorf("create user identity: %w", createErr)
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

func (r *entUserIdentityRepository) WithUserBindingTx(ctx context.Context, fn domain.UserIdentityBindingTxFunc) (*domain.User, error) {
	if fn == nil {
		return nil, fmt.Errorf("user identity transaction function is nil")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start user identity transaction: %w", err)
	}

	txClient := tx.Client()
	userRepo := NewUserRepository(txClient)
	identityRepo := &entUserIdentityRepository{
		client: txClient,
	}

	user, err := fn(ctx, userRepo, identityRepo)
	if err != nil {
		return nil, rollbackUserIdentityTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user identity transaction: %w", err)
	}
	return user, nil
}

func rollbackUserIdentityTx(tx *ent.Tx, err error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, context.Canceled) {
		return fmt.Errorf("%w: rollback user identity transaction: %v", err, rollbackErr)
	}
	return err
}

func (r *entUserIdentityRepository) updateOne(ctx context.Context, id string, account *domain.UserIdentity) error {
	updated, err := r.client.UserIdentity.UpdateOneID(id).
		SetUserID(account.UserID).
		SetEmail(account.Email).
		SetName(account.Name).
		SetAvatarURL(account.AvatarURL).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update user identity: %w", err)
	}
	account.ID = updated.ID
	account.CreatedAt = updated.CreatedAt
	account.UpdatedAt = updated.UpdatedAt
	return nil
}
