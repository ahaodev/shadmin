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

	exists, err := ru.roleRepository.ExistsByName(ctx, request.Name)
	if err != nil {
		return fmt.Errorf("check role name: %w", err)
	}
	if exists {
		return domain.ErrRoleNameExists
	}

	role := &domain.Role{
		Name:     request.Name,
		Sequence: request.Sequence,
		Status:   request.Status,
		MenuIDs:  request.MenuIDs,
	}

	if role.Status == "" {
		role.Status = domain.RoleStatusActive
	}

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

	existingRole, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	if existingRole.IsSystem && request.Name != nil && *request.Name != existingRole.Name {
		return nil, domain.ErrCannotRenameAdminRole
	}

	// 预查重名以返回领域错误，避免唯一约束错误变成 500。
	if request.Name != nil && *request.Name != existingRole.Name {
		exists, err := ru.roleRepository.ExistsByName(ctx, *request.Name)
		if err != nil {
			return nil, fmt.Errorf("check role name: %w", err)
		}
		if exists {
			return nil, domain.ErrRoleNameExists
		}
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

	updatedRole, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated role: %w", err)
	}

	return updatedRole, nil
}

func (ru *roleUsecase) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, ru.contextTimeout)
	defer cancel()

	role, err := ru.roleRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get role: %w", err)
	}

	if role.IsSystem {
		return domain.ErrCannotDeleteAdminRole
	}

	// 使用情况检查与删除在仓储事务内完成。
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
