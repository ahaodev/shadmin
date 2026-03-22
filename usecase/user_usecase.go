package usecase

import (
	"context"
	"fmt"
	"log"
	"shadmin/ent"
	"shadmin/ent/user"
	"time"

	"shadmin/domain"

	"golang.org/x/crypto/bcrypt"
)

type userUsecase struct {
	client         *ent.Client
	userRepository domain.UserRepository
	roleRepository domain.RoleRepository
	contextTimeout time.Duration
}

func NewUserUsecase(client *ent.Client, userRepository domain.UserRepository, roleRepository domain.RoleRepository, timeout time.Duration) domain.UserUseCase {
	return &userUsecase{
		client:         client,
		userRepository: userRepository,
		roleRepository: roleRepository,
		contextTimeout: timeout,
	}
}

func (uu *userUsecase) CreateUser(c context.Context, request *domain.CreateUserRequest) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// 创建用户对象
	user := &domain.User{
		Username: request.Username,
		Email:    request.Email,
		Phone:    request.Phone,
		Password: request.Password,
		Status:   request.Status,
	}

	// 设置默认状态
	if user.Status == "" {
		user.Status = domain.UserStatusActive
	}

	// 🔒 加密密码
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = string(hashedPassword)
	}

	// 创建用户
	if err := uu.userRepository.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 分配角色（如果提供了角色ID）
	if len(request.RoleIDs) > 0 {
		for _, roleID := range request.RoleIDs {
			if err := uu.roleRepository.Assign(ctx, user.ID, roleID); err != nil {
				// 如果角色分配失败，记录错误但不影响用户创建
				log.Printf("Failed to assign role %s to user %s: %v", roleID, user.ID, err)
			}
		}
	}

	log.Printf("INFO: Successfully created user %s", user.Username)
	return user, nil
}
func (uu *userUsecase) ListUsers(c context.Context, filter domain.UserQueryFilter) (*domain.PagedResult[*domain.User], error) {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()
	return uu.userRepository.Query(ctx, filter)
}

func (uu *userUsecase) GetUserByID(c context.Context, id string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()
	return uu.userRepository.GetByID(ctx, id)
}

func (uu *userUsecase) DeleteUser(c context.Context, id string, currentUserID string) error {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// 规则1: 不能删除自己
	if id == currentUserID {
		return domain.ErrCannotDeleteSelf
	}

	// 1. 获取目标用户信息
	user, err := uu.userRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get user for deletion: %w", err)
	}

	// 规则2: 不能删除 isAdmin 管理员
	if user.IsAdmin {
		return domain.ErrCannotDeleteAdmin
	}

	log.Printf("INFO: User %s starting deletion of user %s (ID: %s)", currentUserID, user.Username, id)

	if err := uu.userRepository.Delete(ctx, id); err != nil {
		log.Printf("ERROR: Failed to delete user %s from database: %v", user.Username, err)
		return fmt.Errorf("failed to delete user from database: %w", err)
	}

	log.Printf("INFO: Successfully deleted user %s", user.Username)
	return nil
}

func (uu *userUsecase) UpdateUserProfile(c context.Context, userID string, updates domain.ProfileUpdate) error {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// Get existing user
	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update the user with new data
	if updates.Name != "" {
		user.Username = updates.Name
	}
	// Note: Avatar field is not in the current User struct, but can be added later

	return uu.userRepository.Update(ctx, user)
}

func (uu *userUsecase) UpdateUserPassword(c context.Context, userID string, passwordUpdate domain.PasswordUpdate) error {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// Get existing user
	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwordUpdate.CurrentPassword)); err != nil {
		return domain.ErrInvalidPassword
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordUpdate.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update password
	user.Password = string(hashedPassword)
	return uu.userRepository.Update(ctx, user)
}

func (uu *userUsecase) UpdateUserPartial(c context.Context, userID string, updates domain.UserUpdateRequest) error {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// Get existing user
	user, err := uu.userRepository.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Apply partial updates
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
		// 🔒 加密密码
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*updates.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}

	// Update user basic info first
	err = uu.userRepository.Update(ctx, user)
	if err != nil {
		return err
	}

	// Handle role updates if provided
	if len(updates.RoleIDs) > 0 {
		// Get current user roles
		currentRoles, err := uu.GetUserRoles(ctx, userID)
		if err != nil {
			return err
		}

		// Create maps for easier comparison
		currentRoleMap := make(map[string]bool)
		for _, roleID := range currentRoles {
			currentRoleMap[roleID] = true
		}

		newRoleMap := make(map[string]bool)
		for _, roleID := range updates.RoleIDs {
			newRoleMap[roleID] = true
		}

		// Find roles to add and remove
		for _, roleID := range updates.RoleIDs {
			if !currentRoleMap[roleID] {
				// Add new role
				err = uu.roleRepository.Assign(ctx, userID, roleID)
				if err != nil {
					return err
				}
			}
		}

		for _, roleID := range currentRoles {
			if !newRoleMap[roleID] {
				// Remove old role
				err = uu.roleRepository.Revoke(ctx, userID, roleID)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GetUserRoles 获取用户角色
func (uu *userUsecase) GetUserRoles(c context.Context, userID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(c, uu.contextTimeout)
	defer cancel()

	// 直接从数据库查询用户的角色关系
	user, err := uu.client.User.
		Query().
		Where(user.ID(userID)).
		WithRoles().
		First(ctx)
	if err != nil {
		return nil, err
	}

	// 提取角色ID
	var roleIDs []string
	for _, role := range user.Edges.Roles {
		roleIDs = append(roleIDs, role.ID)
	}

	log.Printf("DEBUG: GetUserRoles - userID: %s, roles: %v", userID, roleIDs)
	return roleIDs, nil
}

// InviteUser 邀请用户 (单租户简化版)
func (uu *userUsecase) InviteUser(c context.Context, request *domain.InviteUserRequest, invitedBy string) (*domain.User, error) {
	// 在单租户架构下，邀请用户就是直接创建用户
	createReq := &domain.CreateUserRequest{
		Username: request.Email, // 使用邮箱作为用户名
		Email:    request.Email,
		RoleIDs:  request.RoleIDs,
	}

	return uu.CreateUser(c, createReq)
}
