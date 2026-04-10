package domain

import (
	"context"
	"errors"
	"time"
)

// Department represents a department in the organization
type Department struct {
	ID        string       `json:"id"`
	ParentID  *string      `json:"parent_id"`
	Name      string       `json:"name"`
	Sequence  int          `json:"sequence"`
	Leader    string       `json:"leader,omitempty"`
	Phone     string       `json:"phone,omitempty"`
	Email     string       `json:"email,omitempty"`
	Status    string       `json:"status"`
	Children  []Department `json:"children,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// CreateDepartmentRequest represents the request to create a new department
type CreateDepartmentRequest struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name" binding:"required,max=100"`
	Sequence int     `json:"sequence"`
	Leader   string  `json:"leader,omitempty" binding:"max=64"`
	Phone    string  `json:"phone,omitempty" binding:"max=20"`
	Email    string  `json:"email,omitempty" binding:"omitempty,email,max=100"`
	Status   string  `json:"status,omitempty"`
}

// UpdateDepartmentRequest represents the request to update a department (pointer fields for partial update)
type UpdateDepartmentRequest struct {
	ParentID *string `json:"parent_id,omitempty"`
	Name     *string `json:"name,omitempty" binding:"omitempty,max=100"`
	Sequence *int    `json:"sequence,omitempty"`
	Leader   *string `json:"leader,omitempty" binding:"omitempty,max=64"`
	Phone    *string `json:"phone,omitempty" binding:"omitempty,max=20"`
	Email    *string `json:"email,omitempty" binding:"omitempty,email,max=100"`
	Status   *string `json:"status,omitempty"`
}

// DepartmentQueryFilter represents query parameters for department filtering
type DepartmentQueryFilter struct {
	Name   string `json:"name,omitempty" form:"name"`
	Status string `json:"status,omitempty" form:"status"`
	Search string `json:"search,omitempty" form:"search"`
	QueryParams
}

// Department sentinel errors
var (
	ErrDepartmentHasChildren = errors.New("该部门下存在子部门，无法删除")
	ErrDepartmentHasUsers    = errors.New("该部门下存在用户，无法删除")
	ErrDepartmentNotFound    = errors.New("部门不存在")
	ErrDepartmentNameExists  = errors.New("同级下已存在同名部门")
	ErrCircularDepartment    = errors.New("不能将部门移动到其子部门下")
)

// DepartmentRepository defines the interface for department data operations
type DepartmentRepository interface {
	Create(ctx context.Context, dept *Department) error
	GetByID(ctx context.Context, id string) (*Department, error)
	FetchTree(ctx context.Context) ([]Department, error)
	FetchList(ctx context.Context, filter DepartmentQueryFilter) ([]Department, error)
	Update(ctx context.Context, dept *Department) error
	Delete(ctx context.Context, id string) error
	HasChildren(ctx context.Context, id string) (bool, error)
	HasUsers(ctx context.Context, id string) (bool, error)
	GetByNameAndParent(ctx context.Context, name string, parentID *string) (*Department, error)
	GetAllChildrenIDs(ctx context.Context, id string) ([]string, error)
}

// DepartmentUseCase defines the interface for department business logic
type DepartmentUseCase interface {
	Create(ctx context.Context, req *CreateDepartmentRequest) error
	GetByID(ctx context.Context, id string) (*Department, error)
	FetchTree(ctx context.Context) ([]Department, error)
	Update(ctx context.Context, id string, req *UpdateDepartmentRequest) (*Department, error)
	Delete(ctx context.Context, id string) error
}
