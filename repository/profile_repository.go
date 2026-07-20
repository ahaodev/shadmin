package repository

import (
	"context"
	"fmt"
	"strings"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/user"
	"shadmin/ent/useridentity"
)

type entProfileRepository struct {
	client *ent.Client
}

func NewProfileRepository(client *ent.Client) domain.ProfileRepository {
	return &entProfileRepository{
		client: client,
	}
}

func (pr *entProfileRepository) GetByID(c context.Context, id, subject string) (*domain.Profile, error) {
	u, err := pr.client.User.
		Query().
		Where(user.ID(id)).
		First(c)

	if err != nil {
		return nil, fmt.Errorf("get profile by id: %w", err)
	}

	// 依据 JWT 的 sub 解析头像来源：
	//   shadmin:<user_id>            → 使用系统内原生头像 user.avatar
	//   <provider>:<provider_subject> → 使用第三方身份头像 user_identity.avatar_url
	avatar, err := pr.resolveAvatar(c, u, subject)
	if err != nil {
		return nil, err
	}

	// Convert ent.User to domain.Profile
	profile := &domain.Profile{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Bio:       u.Bio,
		Avatar:    avatar,
		Status:    string(u.Status),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}

	return profile, nil
}

// resolveAvatar 根据 subject（JWT sub）决定返回哪个头像。
// subject 以第一个冒号切分为 provider 与 providerSubject；provider_subject 自身可能含冒号。
func (pr *entProfileRepository) resolveAvatar(c context.Context, u *ent.User, subject string) (string, error) {
	provider, providerSubject, ok := strings.Cut(subject, ":")
	// 非第三方身份（shadmin 用户或缺失 sub）时，返回系统原生头像。
	if !ok || provider == "" || provider == "shadmin" {
		return u.Avatar, nil
	}

	identity, err := pr.client.UserIdentity.
		Query().
		Where(
			useridentity.Provider(provider),
			useridentity.ProviderSubject(providerSubject),
		).
		Only(c)
	if err != nil {
		if ent.IsNotFound(err) {
			return u.Avatar, nil
		}
		return "", fmt.Errorf("get identity avatar: %w", err)
	}

	return identity.AvatarURL, nil
}

func (pr *entProfileRepository) UpdateProfile(c context.Context, userID string, updateData domain.ProfileUpdate) error {
	updateQuery := pr.client.User.
		UpdateOneID(userID)

	if updateData.Name != "" {
		updateQuery = updateQuery.SetUsername(updateData.Name)
	}
	if updateData.Avatar != "" {
		updateQuery = updateQuery.SetAvatar(updateData.Avatar)
	}
	if updateData.Bio != "" {
		updateQuery = updateQuery.SetBio(updateData.Bio)
	}

	_, err := updateQuery.Save(c)
	return err
}

func (pr *entProfileRepository) UpdatePassword(c context.Context, userID, hashedPassword string) error {
	_, err := pr.client.User.
		UpdateOneID(userID).
		SetPassword(hashedPassword).
		Save(c)
	return err
}

func (pr *entProfileRepository) GetPasswordHash(c context.Context, userID string) (string, error) {
	u, err := pr.client.User.
		Query().
		Where(user.ID(userID)).
		Select(user.FieldPassword).
		First(c)
	if err != nil {
		return "", err
	}
	return u.Password, nil
}
