package domain

import (
	"context"
	"errors"
	"time"
)

const (
	RoleStatusActive   = StatusActive
	RoleStatusInactive = StatusInactive
	RoleNameViewer     = "viewer"
)

var (
	ErrCannotDeleteAdminRole = errors.New("不能删除系统管理员角色")
	ErrCannotRenameAdminRole = errors.New("不能修改系统管理员角色名称")
	ErrRoleNotFound          = errors.New("角色不存在")
	ErrRoleNameExists        = errors.New("角色名称已存在")
	ErrRoleInUse             = errors.New("角色仍被用户使用，无法删除")
)

type Role struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsSystem  bool      `json:"is_system"`
	Sequence  int       `json:"sequence"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MenuIDs   []string  `json:"menus"`
}

type CreateRoleRequest struct {
	Name     string   `json:"name" binding:"required,max=100"`
	Sequence int      `json:"sequence"`
	Status   string   `json:"status,omitempty"`
	MenuIDs  []string `json:"menu_ids,omitempty"`
}

type UpdateRoleRequest struct {
	Name     *string  `json:"name,omitempty"`
	Sequence *int     `json:"sequence,omitempty"`
	Status   *string  `json:"status,omitempty"`
	MenuIDs  []string `json:"menu_ids,omitempty"`
}

type RoleFilters struct {
	Status string `json:"status,omitempty"`
	Name   string `json:"name,omitempty"`
}

type RoleUseCase interface {
	Create(ctx context.Context, request *CreateRoleRequest) error
	Fetch(ctx context.Context) ([]*Role, error)
	GetByID(ctx context.Context, id string) (*Role, error)
	Update(ctx context.Context, id string, request *UpdateRoleRequest) (*Role, error)
	Delete(ctx context.Context, id string) error
}

type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	Fetch(ctx context.Context) ([]*Role, error)
	GetByID(ctx context.Context, id string) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	GetByIDs(ctx context.Context, ids []string) ([]*Role, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	Update(ctx context.Context, role *Role) error
	DeleteIfUnused(ctx context.Context, id, name string) error
}
