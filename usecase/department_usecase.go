package usecase

import (
	"context"
	"fmt"
	"shadmin/domain"
	"slices"
	"time"
)

type departmentUsecase struct {
	departmentRepo domain.DepartmentRepository
	contextTimeout time.Duration
}

func NewDepartmentUsecase(departmentRepo domain.DepartmentRepository, timeout time.Duration) domain.DepartmentUseCase {
	return &departmentUsecase{
		departmentRepo: departmentRepo,
		contextTimeout: timeout,
	}
}

func (u *departmentUsecase) Create(ctx context.Context, req *domain.CreateDepartmentRequest) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// Set default status
	if req.Status == "" {
		req.Status = domain.StatusActive
	}

	// If parent_id is provided, verify parent exists
	if req.ParentID != nil && *req.ParentID != "" {
		_, err := u.departmentRepo.GetByID(ctx, *req.ParentID)
		if err != nil {
			return fmt.Errorf("parent department not found: %w", err)
		}
	}

	dept := &domain.Department{
		ParentID: req.ParentID,
		Name:     req.Name,
		Sequence: req.Sequence,
		Leader:   req.Leader,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	if err := u.departmentRepo.Create(ctx, dept); err != nil {
		return fmt.Errorf("create department: %w", err)
	}
	return nil
}

func (u *departmentUsecase) GetByID(ctx context.Context, id string) (*domain.Department, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.departmentRepo.GetByID(ctx, id)
}

func (u *departmentUsecase) FetchTree(ctx context.Context) ([]domain.Department, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.departmentRepo.FetchTree(ctx)
}

func (u *departmentUsecase) Update(ctx context.Context, id string, req *domain.UpdateDepartmentRequest) (*domain.Department, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	dept, err := u.departmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.ErrDepartmentNotFound
	}

	// Apply partial updates
	if req.Name != nil {
		dept.Name = *req.Name
	}
	if req.Sequence != nil {
		dept.Sequence = *req.Sequence
	}
	if req.Leader != nil {
		dept.Leader = *req.Leader
	}
	if req.Phone != nil {
		dept.Phone = *req.Phone
	}
	if req.Email != nil {
		dept.Email = *req.Email
	}
	if req.Status != nil {
		// If disabling, ensure no active descendants exist
		if *req.Status == domain.StatusInactive && dept.Status == domain.StatusActive {
			hasActive, err := u.departmentRepo.HasActiveChildren(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("check active children: %w", err)
			}
			if hasActive {
				return nil, domain.ErrDepartmentHasActiveChildren
			}
		}
		dept.Status = *req.Status
	}

	// Handle parent_id change — check circular reference
	if req.ParentID != nil {
		newParentID := req.ParentID
		if *newParentID == "" {
			dept.ParentID = nil
		} else {
			// Cannot set parent to self
			if *newParentID == id {
				return nil, domain.ErrCircularDepartment
			}
			// Cannot set parent to own descendant
			childIDs, err := u.departmentRepo.GetAllChildrenIDs(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("check children: %w", err)
			}
			if slices.Contains(childIDs, *newParentID) {
				return nil, domain.ErrCircularDepartment
			}
			dept.ParentID = newParentID
		}
	}

	if err := u.departmentRepo.Update(ctx, dept); err != nil {
		return nil, fmt.Errorf("update department: %w", err)
	}
	return dept, nil
}

func (u *departmentUsecase) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// Check existence
	_, err := u.departmentRepo.GetByID(ctx, id)
	if err != nil {
		return domain.ErrDepartmentNotFound
	}

	// Check children
	has, err := u.departmentRepo.HasChildren(ctx, id)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if has {
		return domain.ErrDepartmentHasChildren
	}

	// Check users
	has, err = u.departmentRepo.HasUsers(ctx, id)
	if err != nil {
		return fmt.Errorf("check users: %w", err)
	}
	if has {
		return domain.ErrDepartmentHasUsers
	}

	if err := u.departmentRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete department: %w", err)
	}
	return nil
}
