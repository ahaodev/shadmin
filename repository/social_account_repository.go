package repository

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/socialaccount"
)

type entSocialAccountRepository struct {
	client *ent.Client
}

// NewSocialAccountRepository 构造第三方账号绑定的 ent 仓储实现
func NewSocialAccountRepository(client *ent.Client) domain.SocialAccountRepository {
	return &entSocialAccountRepository{client: client}
}

// entSocialAccountToDomain 把 ent.SocialAccount 转成 domain.SocialAccount
func entSocialAccountToDomain(a *ent.SocialAccount) *domain.SocialAccount {
	if a == nil {
		return nil
	}
	return &domain.SocialAccount{
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
// 未找到时返回 (nil, nil)，调用方据此决定是否走“按邮箱匹配/创建新用户”分支。
func (r *entSocialAccountRepository) FindByProviderAndSubject(ctx context.Context, provider, subject string) (*domain.SocialAccount, error) {
	if provider == "" || subject == "" {
		return nil, nil
	}

	a, err := r.client.SocialAccount.Query().
		Where(
			socialaccount.Provider(provider),
			socialaccount.ProviderSubject(subject),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query social account: %w", err)
	}
	return entSocialAccountToDomain(a), nil
}

// FindByUserID 查询某用户绑定的全部第三方账号
func (r *entSocialAccountRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.SocialAccount, error) {
	if userID == "" {
		return nil, nil
	}

	accounts, err := r.client.SocialAccount.Query().
		Where(socialaccount.UserID(userID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query social accounts by user id: %w", err)
	}

	result := make([]*domain.SocialAccount, 0, len(accounts))
	for _, a := range accounts {
		result = append(result, entSocialAccountToDomain(a))
	}
	return result, nil
}

// Upsert 按 (provider, provider_subject) 创建或更新绑定。
// 绑定存在时更新 user_id/email/name/avatar_url，不存在时新建。
// 这里的“创建或更新”用先 Query 再分支，避免依赖具体方言的 ON CONFLICT。
func (r *entSocialAccountRepository) Upsert(ctx context.Context, account *domain.SocialAccount) error {
	if account == nil {
		return fmt.Errorf("social account is nil")
	}
	if account.Provider == "" || account.ProviderSubject == "" {
		return fmt.Errorf("provider and provider_subject are required")
	}

	existing, err := r.client.SocialAccount.Query().
		Where(
			socialaccount.Provider(account.Provider),
			socialaccount.ProviderSubject(account.ProviderSubject),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("query existing social account: %w", err)
	}

	if ent.IsNotFound(err) || existing == nil {
		create := r.client.SocialAccount.Create().
			SetUserID(account.UserID).
			SetProvider(account.Provider).
			SetProviderSubject(account.ProviderSubject).
			SetEmail(account.Email).
			SetName(account.Name).
			SetAvatarURL(account.AvatarURL)
		created, err := create.Save(ctx)
		if err != nil {
			// 并发场景下另一个请求可能已经创建，再次查询后走更新分支
			if ent.IsConstraintError(err) {
				return r.updateExisting(ctx, account)
			}
			return fmt.Errorf("create social account: %w", err)
		}
		account.ID = created.ID
		account.CreatedAt = created.CreatedAt
		account.UpdatedAt = created.UpdatedAt
		return nil
	}

	return r.updateOne(ctx, existing.ID, account)
}

func (r *entSocialAccountRepository) updateExisting(ctx context.Context, account *domain.SocialAccount) error {
	existing, err := r.client.SocialAccount.Query().
		Where(
			socialaccount.Provider(account.Provider),
			socialaccount.ProviderSubject(account.ProviderSubject),
		).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("re-query social account for update: %w", err)
	}
	return r.updateOne(ctx, existing.ID, account)
}

func (r *entSocialAccountRepository) updateOne(ctx context.Context, id string, account *domain.SocialAccount) error {
	updated, err := r.client.SocialAccount.UpdateOneID(id).
		SetUserID(account.UserID).
		SetEmail(account.Email).
		SetName(account.Name).
		SetAvatarURL(account.AvatarURL).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update social account: %w", err)
	}
	account.ID = updated.ID
	account.CreatedAt = updated.CreatedAt
	account.UpdatedAt = updated.UpdatedAt
	return nil
}
