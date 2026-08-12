package usecase

import (
	"context"
	"fmt"
	"time"

	"shadmin/domain"
	"shadmin/pkg"
)

type resourceUsecase struct {
	menuRepository domain.MenuRepository
	userRepository domain.UserRepository
	roleRepository domain.RoleRepository
	contextTimeout time.Duration
}

func NewResourceUsecase(menuRepository domain.MenuRepository, userRepository domain.UserRepository, roleRepository domain.RoleRepository, timeout time.Duration) domain.ResourceUseCase {
	return &resourceUsecase{
		menuRepository: menuRepository,
		userRepository: userRepository,
		roleRepository: roleRepository,
		contextTimeout: timeout,
	}
}

// GetUserResources 获取用户可访问的系统资源（菜单树与按钮权限，基于 RBAC 权限过滤）
func (ru *resourceUsecase) GetUserResources(c context.Context, userID string, isAdmin bool) (*domain.UserResources, error) {
	ctx, cancel := context.WithTimeout(c, ru.contextTimeout)
	defer cancel()

	// 获取所有菜单
	menuTree, err := ru.menuRepository.GetMenuTree(ctx)
	if err != nil {
		pkg.Log.WithField("userID", userID).WithError(err).Error("Failed to retrieve menu tree")
		return nil, fmt.Errorf("failed to retrieve menu tree: %w", err)
	}

	// 获取用户信息（用于解析角色名称）
	user, err := ru.userRepository.GetByID(ctx, userID)
	if err != nil {
		pkg.Log.WithField("userID", userID).WithError(err).Error("Failed to get user information")
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 管理员直接返回所有菜单和权限
	if isAdmin {
		adminRoleObjects, _ := ru.getUserRoles(ctx, user.Roles)
		adminRoleNames := make([]string, 0, len(adminRoleObjects))
		for _, r := range adminRoleObjects {
			adminRoleNames = append(adminRoleNames, r.Name)
		}

		return &domain.UserResources{
			Menus:       menuTree,
			Permissions: ru.collectAllPermissions(menuTree),
			Roles:       adminRoleNames,
			IsAdmin:     true,
		}, nil
	}

	// 非管理员：按角色过滤菜单与权限
	roles, err := ru.getUserRoles(ctx, user.Roles)
	if err != nil {
		return nil, err
	}

	userMenuIDs, userPermissions := ru.collectUserPermissions(ctx, roles)

	// 提取角色名称列表
	roleNames := make([]string, 0, len(roles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Name)
	}

	return &domain.UserResources{
		Menus:       ru.filterMenuTree(menuTree, userMenuIDs),
		Permissions: userPermissions,
		Roles:       roleNames,
		IsAdmin:     false,
	}, nil
}

// collectAllPermissions 收集所有按钮权限
func (ru *resourceUsecase) collectAllPermissions(menuTree []domain.MenuTreeNode) []string {
	var permissions []string
	permissionMap := make(map[string]bool)

	var collect func([]domain.MenuTreeNode)
	collect = func(nodes []domain.MenuTreeNode) {
		for _, node := range nodes {
			if node.Type == domain.MenuTypeButton && node.Permissions != nil && *node.Permissions != "" {
				if !permissionMap[*node.Permissions] {
					permissionMap[*node.Permissions] = true
					permissions = append(permissions, *node.Permissions)
				}
			}
			collect(node.Children)
		}
	}
	collect(menuTree)
	return permissions
}

// getUserRoles 批量获取用户角色信息
func (ru *resourceUsecase) getUserRoles(c context.Context, roleIDs []string) ([]*domain.Role, error) {
	var roles []*domain.Role
	var failedRoles []string

	for _, roleID := range roleIDs {
		role, err := ru.roleRepository.GetByID(c, roleID)
		if err != nil {
			failedRoles = append(failedRoles, roleID)
			pkg.Log.WithField("roleID", roleID).WithError(err).Warn("Failed to get role information")
			continue
		}
		roles = append(roles, role)
	}

	if len(failedRoles) > 0 {
		pkg.Log.WithField("failedRoles", failedRoles).Warn("Some roles could not be retrieved")
	}

	return roles, nil
}

// collectUserPermissions 收集用户菜单ID和权限
func (ru *resourceUsecase) collectUserPermissions(c context.Context, roles []*domain.Role) ([]string, []string) {
	var userMenuIDs []string
	var userPermissions []string
	permissionMap := make(map[string]bool)

	// 收集所有菜单ID
	for _, role := range roles {
		userMenuIDs = append(userMenuIDs, role.MenusIds...)
	}

	// 收集权限标识
	var failedMenus []string
	for _, role := range roles {
		for _, menuID := range role.MenusIds {
			menu, err := ru.menuRepository.GetMenuByID(c, menuID)
			if err != nil {
				failedMenus = append(failedMenus, menuID)
				pkg.Log.WithField("menuID", menuID).WithError(err).Warn("Failed to get menu information")
				continue
			}
			if menu.Type == domain.MenuTypeButton && menu.Permissions != nil && *menu.Permissions != "" {
				if !permissionMap[*menu.Permissions] {
					permissionMap[*menu.Permissions] = true
					userPermissions = append(userPermissions, *menu.Permissions)
				}
			}
		}
	}

	if len(failedMenus) > 0 {
		pkg.Log.WithField("failedMenus", failedMenus).Warn("Some menus could not be retrieved for permission collection")
	}

	return userMenuIDs, userPermissions
}

// filterMenuTree 过滤菜单树
func (ru *resourceUsecase) filterMenuTree(menuTree []domain.MenuTreeNode, userMenuIDs []string) []domain.MenuTreeNode {
	allowedIDsMap := make(map[string]bool)
	for _, id := range userMenuIDs {
		allowedIDsMap[id] = true
	}

	var filteredMenus []domain.MenuTreeNode
	for _, menu := range menuTree {
		if filteredMenu := ru.filterMenuNodeByIDs(menu, allowedIDsMap); filteredMenu != nil {
			filteredMenus = append(filteredMenus, *filteredMenu)
		}
	}
	return filteredMenus
}

// filterMenuNodeByIDs 递归过滤单个菜单节点及其子节点
func (ru *resourceUsecase) filterMenuNodeByIDs(menu domain.MenuTreeNode, allowedIDsMap map[string]bool) *domain.MenuTreeNode {
	// 递归过滤子菜单
	var filteredChildren []domain.MenuTreeNode
	for _, child := range menu.Children {
		if filteredChild := ru.filterMenuNodeByIDs(child, allowedIDsMap); filteredChild != nil {
			filteredChildren = append(filteredChildren, *filteredChild)
		}
	}

	// 如果当前菜单有权限或者有有权限的子菜单，则保留
	hasPermission := allowedIDsMap[menu.ID]
	hasAccessibleChildren := len(filteredChildren) > 0

	if hasPermission || hasAccessibleChildren {
		filteredMenu := menu
		filteredMenu.Children = filteredChildren
		return &filteredMenu
	}

	return nil
}
