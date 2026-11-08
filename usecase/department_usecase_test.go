package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"shadmin/domain"
)

type departmentTestRepository struct {
	domain.DepartmentRepository
	dept          *domain.Department
	updateCalls   int
	getByIDErrors map[string]error
}

func (r *departmentTestRepository) GetByID(_ context.Context, id string) (*domain.Department, error) {
	if err, ok := r.getByIDErrors[id]; ok {
		return nil, err
	}
	dept := *r.dept
	return &dept, nil
}

func (r *departmentTestRepository) GetAllChildrenIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func (r *departmentTestRepository) Update(context.Context, *domain.Department) error {
	r.updateCalls++
	return nil
}

func TestDepartmentUsecaseUpdateRejectsMissingParent(t *testing.T) {
	repo := &departmentTestRepository{
		dept:          &domain.Department{ID: "dept-1", Name: "child", Status: domain.StatusActive},
		getByIDErrors: map[string]error{"missing": domain.ErrDepartmentNotFound},
	}
	useCase := NewDepartmentUsecase(repo, time.Second)
	missing := "missing"

	_, err := useCase.Update(context.Background(), "dept-1", &domain.UpdateDepartmentRequest{ParentID: &missing})
	if !errors.Is(err, domain.ErrDepartmentNotFound) {
		t.Fatalf("Update with missing parent = %v; want ErrDepartmentNotFound", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repository updates = %d; want no write", repo.updateCalls)
	}
}
