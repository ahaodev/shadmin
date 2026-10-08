package domain

import "context"

// AuthorizationSource 为 Casbin 快照提供权限事实（由 repository 实现）。
type AuthorizationSource interface {
	// Generation 返回已提交的授权代数。
	Generation(ctx context.Context) (int64, error)
	// ActiveUserRoles 返回所有启用用户及其启用角色 ID。
	ActiveUserRoles(ctx context.Context) ([]UserRoleBinding, error)
	// ActiveRoles 返回所有启用角色及其菜单下的 API 资源。
	ActiveRoles(ctx context.Context) ([]AuthorizedRole, error)
}

// UserRoleBinding 用户与其启用角色的关联。
type UserRoleBinding struct {
	UserID  string
	RoleIDs []string
}

// AuthorizedRole 启用角色及其可访问的 API 资源。
type AuthorizedRole struct {
	ID        string
	Name      string
	Resources []AuthorizedResource
}

// AuthorizedResource 一个 API 资源（路径 + 方法），IsPublic 表示无需授权即可访问。
type AuthorizedResource struct {
	Method   string
	Path     string
	IsPublic bool
}
