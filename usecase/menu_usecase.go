package usecase

import (
	"context"
	"fmt"
	"slices"
	"time"

	"shadmin/domain"
)

type menuUsecase struct {
	menuRepository domain.MenuRepository
	contextTimeout time.Duration
}

func NewMenuUsecase(menuRepository domain.MenuRepository, timeout time.Duration) domain.MenuUseCase {
	return &menuUsecase{
		menuRepository: menuRepository,
		contextTimeout: timeout,
	}
}

// GetMenuTree Query operations
func (mu *menuUsecase) GetMenuTree(ctx context.Context) ([]domain.MenuTreeNode, error) {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	return mu.menuRepository.GetMenuTree(ctx)
}
func (mu *menuUsecase) GetMenus(ctx context.Context, params domain.MenuQueryParams) (*domain.PagedResult[domain.Menu], error) {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	return mu.menuRepository.GetMenus(ctx, params)
}

func (mu *menuUsecase) GetMenuByID(ctx context.Context, id string) (*domain.Menu, error) {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	return mu.menuRepository.GetMenuByID(ctx, id)
}

// CRUD operations
func (mu *menuUsecase) CreateMenu(ctx context.Context, req *domain.CreateMenuRequest) (*domain.Menu, error) {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	// Validate menu type
	validTypes := []string{domain.MenuTypeMenu, domain.MenuTypeButton}
	isValidType := slices.Contains(validTypes, req.Type)
	if !isValidType {
		return nil, domain.ErrInvalidMenuType
	}

	// Set default status if not provided
	if req.Status == "" {
		req.Status = domain.MenuStatusActive
	}

	// Validate status
	if req.Status != domain.MenuStatusActive && req.Status != domain.MenuStatusInactive {
		return nil, domain.ErrInvalidMenuStatus
	}

	// Create menu in repository
	createdMenu, err := mu.menuRepository.CreateMenu(ctx, req)
	if err != nil {
		return nil, err
	}

	return createdMenu, nil
}

func (mu *menuUsecase) UpdateMenu(ctx context.Context, id string, req *domain.UpdateMenuRequest) (*domain.Menu, error) {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	// Validate menu type
	validTypes := []string{domain.MenuTypeMenu, domain.MenuTypeButton}
	isValidType := slices.Contains(validTypes, req.Type)
	if !isValidType {
		return nil, domain.ErrInvalidMenuType
	}

	// Validate status
	if req.Status != domain.MenuStatusActive && req.Status != domain.MenuStatusInactive {
		return nil, domain.ErrInvalidMenuStatus
	}

	// Update menu in repository
	updatedMenu, err := mu.menuRepository.UpdateMenu(ctx, id, req)
	if err != nil {
		return nil, err
	}

	return updatedMenu, nil
}

func (mu *menuUsecase) DeleteMenu(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, mu.contextTimeout)
	defer cancel()

	// 先确认菜单存在，再递归删除菜单及其所有子菜单；事务由 repository 处理
	if _, err := mu.menuRepository.GetMenuByID(ctx, id); err != nil {
		return fmt.Errorf("get menu: %w", err)
	}
	if err := mu.menuRepository.DeleteMenuTree(ctx, id); err != nil {
		return fmt.Errorf("failed to delete menu recursively: %w", err)
	}
	return nil
}
