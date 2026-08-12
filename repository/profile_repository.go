package repository

import (
	"context"
	"fmt"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/user"
)

type entProfileRepository struct {
	client *ent.Client
}

func NewProfileRepository(client *ent.Client) domain.ProfileRepository {
	return &entProfileRepository{
		client: client,
	}
}

func (pr *entProfileRepository) GetByID(c context.Context, id, _ string) (*domain.Profile, error) {
	u, err := pr.client.User.
		Query().
		Where(user.ID(id)).
		First(c)

	if err != nil {
		return nil, fmt.Errorf("get profile by id: %w", err)
	}

	// 头像统一取自 users.avatar：oidc 用户资料已由登录流程同步到 users 表。
	profile := &domain.Profile{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Email:     derefString(u.Email),
		Phone:     derefString(u.Phone),
		Bio:       u.Bio,
		Avatar:    u.Avatar,
		Status:    string(u.Status),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}

	return profile, nil
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
	return derefString(u.Password), nil
}
