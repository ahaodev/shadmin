package domain

import "context"

// UserResources 用户可访问的系统资源（菜单树 + 按钮权限 + 角色名）
type UserResources struct {
	Menus       []MenuTreeNode `json:"menus"`
	Permissions []string       `json:"permissions"`
	Roles       []string       `json:"roles"`
	IsAdmin     bool           `json:"is_admin"`
}

// ResourceUseCase 定义资源（菜单/权限）业务逻辑
type ResourceUseCase interface {
	GetUserResources(ctx context.Context, userID string, isAdmin bool) (*UserResources, error)
}
