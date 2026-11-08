package usecase

import (
	"context"
	"shadmin/domain"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type profileUsecase struct {
	profileRepo domain.ProfileRepository
	timeout     time.Duration
}

func NewProfileUsecase(profileRepo domain.ProfileRepository, timeout time.Duration) domain.ProfileUsecase {
	return &profileUsecase{
		profileRepo: profileRepo,
		timeout:     timeout,
	}
}

func (pu *profileUsecase) GetProfile(ctx context.Context, userID, subject string) (*domain.Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, pu.timeout)
	defer cancel()

	profile, err := pu.profileRepo.GetByID(ctx, userID, subject)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (pu *profileUsecase) UpdateProfile(ctx context.Context, userID string, updateData domain.ProfileUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, pu.timeout)
	defer cancel()

	return pu.profileRepo.UpdateProfile(ctx, userID, updateData)
}

func (pu *profileUsecase) UpdatePassword(ctx context.Context, userID string, passwordUpdate domain.PasswordUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, pu.timeout)
	defer cancel()

	currentHash, err := pu.profileRepo.GetPasswordHash(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(passwordUpdate.CurrentPassword)); err != nil {
		return domain.ErrInvalidPassword
	}
	if err := domain.ValidateNewPassword(passwordUpdate.NewPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordUpdate.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return pu.profileRepo.UpdatePassword(ctx, userID, string(hashedPassword))
}
