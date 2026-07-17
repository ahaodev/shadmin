package domain

import (
	"context"
	"time"
)

type Profile struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	Bio               string    `json:"bio"`
	Avatar            string    `json:"avatar"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ProviderAvatarURL string    `json:"providerAvatarUrl,omitempty"`
}
type ProfileUsecase interface {
	GetProfile(c context.Context, userID string) (*Profile, error)
	UpdateProfile(c context.Context, userID string, updateData ProfileUpdate) error
	UpdatePassword(c context.Context, userID string, passwordUpdate PasswordUpdate) error
}

type ProfileRepository interface {
	GetByID(c context.Context, id string) (*Profile, error)
	UpdateProfile(c context.Context, userID string, updateData ProfileUpdate) error
	UpdatePassword(c context.Context, userID, hashedPassword string) error
	GetPasswordHash(c context.Context, userID string) (string, error)
}
