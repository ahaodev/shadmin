package domain

import "context"

// PermissionChecker verifies the current authorization generation before evaluating access.
type PermissionChecker interface {
	CheckPermission(ctx context.Context, userID, object, action string) (bool, error)
}

type AuthorizationSource interface {
	Generation(ctx context.Context) (int64, error)
	// ActiveUserRoles 返回所有启用用户及其启用角色 ID。
	ActiveUserRoles(ctx context.Context) ([]UserRoleBinding, error)
	// ActiveRoles 返回所有启用角色及其菜单下的 API 资源。
	ActiveRoles(ctx context.Context) ([]AuthorizedRole, error)
}

type UserRoleBinding struct {
	UserID  string
	RoleIDs []string
}

type AuthorizedRole struct {
	ID        string
	Name      string
	Resources []AuthorizedResource
}

type AuthorizedResource struct {
	Method   string
	Path     string
	IsPublic bool
}
