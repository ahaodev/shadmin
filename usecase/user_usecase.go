package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shadmin/domain"

	"golang.org/x/crypto/bcrypt"
)

type userUsecase struct {
	userRepository domain.UserRepository
	contextTimeout time.Duration
}

func NewUserUsecase(userRepository domain.UserRepository, timeout time.Duration) domain.UserUseCase {
	return &userUsecase{
		userRepository: userRepository,
		contextTimeout: timeout,
	}
}

func (uu *userUsecase) CreateUser(ctx context.Context, request *domain.CreateUserRequest) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()
	if err := domain.ValidateNewPassword(request.Password); err != nil {
		return nil, err
	}

	user := &domain.User{
		Username:     request.Username,
		Email:        request.Email,
		Phone:        request.Phone,
		Password:     request.Password,
		Status:       request.Status,
		DepartmentID: request.DepartmentID,
	}

	if user.Status == "" {
		user.Status = domain.UserStatusActive
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedPassword)

	// 创建用户与角色关联在同一授权事务中提交。
	if err := uu.userRepository.CreateWithRoles(ctx, user, request.RoleIDs); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}
func (uu *userUsecase) ListUsers(ctx context.Context, filter domain.UserQueryFilter) (*domain.PagedResult[*domain.User], error) {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()
	return uu.userRepository.Query(ctx, filter)
}

func (uu *userUsecase) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()
	return uu.userRepository.GetByID(ctx, id)
}

func (uu *userUsecase) DeleteUser(ctx context.Context, id string, currentUserID string) error {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()

	if id == currentUserID {
		return domain.ErrCannotDeleteSelf
	}

	user, err := uu.userRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get user for deletion: %w", err)
	}

	if user.IsAdmin {
		return domain.ErrCannotDeleteAdmin
	}

	slog.Info("user deletion started", "operator_id", currentUserID, "username", user.Username, "user_id", id)

	if err := uu.userRepository.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user from database: %w", err)
	}
	return nil
}

func (uu *userUsecase) UpdateUserProfile(ctx context.Context, userID string, updates domain.ProfileUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()

	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.IsAdmin {
		return domain.ErrCannotEditAdmin
	}

	if updates.Name != "" {
		user.Username = updates.Name
	}

	return uu.userRepository.Update(ctx, user)
}

func (uu *userUsecase) UpdateUserPassword(ctx context.Context, userID string, passwordUpdate domain.PasswordUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()

	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.IsAdmin {
		return domain.ErrCannotEditAdmin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwordUpdate.CurrentPassword)); err != nil {
		return domain.ErrInvalidPassword
	}
	if err := domain.ValidateNewPassword(passwordUpdate.NewPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordUpdate.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return uu.userRepository.Update(ctx, user)
}

func (uu *userUsecase) UpdateUserPartial(ctx context.Context, userID string, updates domain.UserUpdateRequest) error {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()

	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.IsAdmin {
		return domain.ErrCannotEditAdmin
	}

	if updates.Username != nil {
		user.Username = *updates.Username
	}
	if updates.Email != nil {
		user.Email = *updates.Email
	}
	if updates.Phone != nil {
		user.Phone = *updates.Phone
	}
	if updates.Status != nil {
		user.Status = *updates.Status
	}
	if updates.Avatar != nil {
		user.Avatar = *updates.Avatar
	}
	if updates.Password != nil && *updates.Password != "" {
		if err := domain.ValidateNewPassword(*updates.Password); err != nil {
			return err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*updates.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}
	if updates.DepartmentID != nil {
		user.DepartmentID = updates.DepartmentID
	}

	if updates.RoleIDs != nil {
		// 空数组清空角色，未传则保留。
		return uu.userRepository.UpdateWithRoles(ctx, user, updates.RoleIDs)
	}
	if updates.Status != nil {
		return uu.userRepository.UpdateWithAuthorization(ctx, user)
	}
	return uu.userRepository.Update(ctx, user)
}

func (uu *userUsecase) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, uu.contextTimeout)
	defer cancel()

	roleIDs, err := uu.userRepository.GetRoleIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	return roleIDs, nil
}
