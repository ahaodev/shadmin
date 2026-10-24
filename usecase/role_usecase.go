package usecase

import (
	"context"
	"fmt"
	"shadmin/domain"
	"time"
)

type roleUsecase struct {
	roleRepository domain.RoleRepository
	contextTimeout time.Duration
}

func (ru *roleUsecase) Create(ctx context.Context, request *domain.CreateRoleRequest) error {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	// Check if role with same name already exists
	exists, err := ru.roleRepository.ExistsByName(ctx, request.Name)
	if err != nil {
		return fmt.Errorf("check role name: %w", err)
	}
	if exists {
		return domain.ErrRoleNameExists
	}

	// Create the role domain model
	role := &domain.Role{
		Name:     request.Name,
		Sequence: request.Sequence,
		Status:   request.Status,
		MenuIDs:  request.MenuIDs,
	}

	// Set default status if not provided
	if role.Status == "" {
		role.Status = domain.RoleStatusActive
	}

	// Create the role
	err = ru.roleRepository.Create(ctx, role)
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	return nil
}

func (ru *roleUsecase) Fetch(ctx context.Context) ([]*domain.Role, error) {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	return ru.roleRepository.Fetch(ctx)
}

func (ru *roleUsecase) GetByID(ctx context.Context, id string) (*domain.Role, error) {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	return ru.roleRepository.GetByID(ctx, id)
}

func (ru *roleUsecase) Update(ctx context.Context, id string, request *domain.UpdateRoleRequest) (*domain.Role, error) {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	// Get existing role
	existingRole, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// 系统内置角色名称受保护，不允许修改
	if existingRole.IsSystem && request.Name != nil && *request.Name != existingRole.Name {
		return nil, domain.ErrCannotRenameAdminRole
	}

	if request.Name != nil {
		existingRole.Name = *request.Name
	}

	if request.Sequence != nil {
		existingRole.Sequence = *request.Sequence
	}

	if request.Status != nil {
		existingRole.Status = *request.Status
	}

	if request.MenuIDs != nil {
		existingRole.MenuIDs = request.MenuIDs
	}

	err = ru.roleRepository.Update(ctx, existingRole)
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	// Fetch and return updated role
	updatedRole, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated role: %w", err)
	}

	return updatedRole, nil
}

func (ru *roleUsecase) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	// 1. 检查角色是否存在
	role, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get role: %w", err)
	}

	// 系统内置角色受保护，不允许删除
	if role.IsSystem {
		return domain.ErrCannotDeleteAdminRole
	}

	// 2. 删除角色记录；使用情况检查和删除在 repository 事务内完成
	if err := ru.roleRepository.DeleteIfUnused(ctx, id, role.Name); err != nil {
		return fmt.Errorf("failed to delete role from database: %w", err)
	}

	return nil
}

func NewRoleUsecase(roleRepository domain.RoleRepository, timeout time.Duration) domain.RoleUseCase {
	return &roleUsecase{
		roleRepository: roleRepository,
		contextTimeout: timeout,
	}
}
