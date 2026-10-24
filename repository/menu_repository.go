package repository

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/menu"
	"strings"
)

type entMenuRepository struct {
	client *ent.Client
}

func NewMenuRepository(client *ent.Client) domain.MenuRepository {
	return &entMenuRepository{
		client: client,
	}
}

// entMenuToDomain 把 ent.Menu 映射为 domain.Menu。
func entMenuToDomain(m *ent.Menu) *domain.Menu {
	var apiResourceIDs []string
	for _, apiResource := range m.Edges.APIResources {
		apiResourceIDs = append(apiResourceIDs, apiResource.ID)
	}

	return &domain.Menu{
		ID:           m.ID,
		Name:         m.Name,
		Sequence:     m.Sequence,
		Type:         m.Type,
		Path:         emptyToNil(m.Path),
		Icon:         m.Icon,
		Component:    emptyToNil(m.Component),
		RouteName:    emptyToNil(m.RouteName),
		Query:        emptyToNil(m.Query),
		IsFrame:      m.IsFrame,
		Visible:      m.Visible,
		Permissions:  emptyToNil(m.Permissions),
		Status:       m.Status,
		ParentID:     m.ParentID,
		ApiResources: apiResourceIDs,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// GetMenuTree retrieves menu tree structure
func (mr *entMenuRepository) GetMenuTree(ctx context.Context) ([]domain.MenuTreeNode, error) {
	// Query all active menus, ordered by sequence, include API resources
	menus, err := clientFromContext(ctx, mr.client).Menu.
		Query().
		Where(menu.Status(domain.MenuStatusActive)).
		WithAPIResources().
		Order(menu.BySequence()).
		All(ctx)

	if err != nil {
		return nil, err
	}

	// Convert ent.Menu to domain.MenuTreeNode
	var allNodes []domain.MenuTreeNode
	for _, m := range menus {
		node := domain.MenuTreeNode{
			ID:          m.ID,
			Name:        m.Name,
			Sequence:    m.Sequence,
			Type:        m.Type,
			Path:        emptyToNil(m.Path),
			Icon:        m.Icon,
			Component:   emptyToNil(m.Component),
			RouteName:   emptyToNil(m.RouteName),
			Query:       emptyToNil(m.Query),
			IsFrame:     m.IsFrame,
			Visible:     m.Visible,
			Permissions: emptyToNil(m.Permissions),
			Status:      m.Status,
			ParentID:    m.ParentID,
			Children:    []domain.MenuTreeNode{},
		}
		allNodes = append(allNodes, node)
	}

	// Build tree structure
	return buildTree(allNodes,
		func(n domain.MenuTreeNode) string { return n.ID },
		func(n domain.MenuTreeNode) *string { return n.ParentID },
		func(n *domain.MenuTreeNode, children []domain.MenuTreeNode) { n.Children = children },
	), nil
}

// GetMenus 返回匹配的全部菜单（菜单是有限集合，不分页），响应仍使用 PagedResult 以保持契约。
func (mr *entMenuRepository) GetMenus(ctx context.Context, params domain.MenuQueryParams) (*domain.PagedResult[domain.Menu], error) {
	query := clientFromContext(ctx, mr.client).Menu.Query()

	// keyword：模糊匹配 name / path
	kw := strings.TrimSpace(params.Keyword)
	if kw != "" {
		query = query.Where(menu.Or(
			menu.NameContains(kw),
			menu.PathContains(kw),
		))
	}

	menus, err := query.
		WithAPIResources().
		Order(menu.BySequence(), menu.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to domain menus
	result := make([]domain.Menu, 0, len(menus))
	for _, m := range menus {
		result = append(result, *entMenuToDomain(m))
	}

	return domain.NewPagedResult(result, len(result), 1, max(len(result), 1)), nil
}

// GetMenuByID retrieves a menu by its ID
func (mr *entMenuRepository) GetMenuByID(ctx context.Context, id string) (*domain.Menu, error) {
	m, err := clientFromContext(ctx, mr.client).Menu.
		Query().
		Where(menu.ID(id)).
		WithAPIResources().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrMenuNotFound
		}
		return nil, err
	}

	return entMenuToDomain(m), nil
}

// GetMenuPermissionsByIDs 批量按 ID 查询菜单，只取"是否按钮 + 权限标识"判断所需字段。
func (mr *entMenuRepository) GetMenuPermissionsByIDs(ctx context.Context, ids []string) ([]*domain.Menu, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	menus, err := clientFromContext(ctx, mr.client).Menu.
		Query().
		Where(menu.IDIn(ids...)).
		Select(menu.FieldID, menu.FieldType, menu.FieldPermissions).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Menu, 0, len(menus))
	for _, m := range menus {
		result = append(result, &domain.Menu{
			ID:          m.ID,
			Type:        m.Type,
			Permissions: emptyToNil(m.Permissions),
		})
	}
	return result, nil
}

// CreateMenu creates a new menu
func (mr *entMenuRepository) CreateMenu(ctx context.Context, req *domain.CreateMenuRequest) (*domain.Menu, error) {
	var createdID string
	err := WithAuthorizationTx(ctx, mr.client, func(txCtx context.Context, tx *ent.Tx) error {
		createQuery := tx.Menu.
			Create().
			SetName(req.Name).
			SetSequence(req.Sequence).
			SetType(req.Type).
			SetNillablePath(req.Path).
			SetIcon(req.Icon).
			SetNillableComponent(req.Component).
			SetNillableRouteName(req.RouteName).
			SetNillableQuery(req.Query).
			SetIsFrame(req.IsFrame).
			SetVisible(req.Visible).
			SetNillablePermissions(req.Permissions).
			SetStatus(req.Status).
			SetNillableParentID(req.ParentID)

		// Handle API resource associations
		if len(req.ApiResources) > 0 {
			createQuery = createQuery.AddAPIResourceIDs(req.ApiResources...)
		}

		created, err := createQuery.Save(txCtx)
		if err != nil {
			return err
		}
		createdID = created.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Fetch created menu with API resources
	return mr.GetMenuByID(ctx, createdID)
}

// UpdateMenu updates an existing menu
func (mr *entMenuRepository) UpdateMenu(ctx context.Context, id string, req *domain.UpdateMenuRequest) (*domain.Menu, error) {
	err := WithAuthorizationTx(ctx, mr.client, func(txCtx context.Context, tx *ent.Tx) error {
		updateQuery := tx.Menu.
			UpdateOneID(id).
			SetName(req.Name).
			SetSequence(req.Sequence).
			SetType(req.Type).
			SetNillablePath(req.Path).
			SetIcon(req.Icon).
			SetNillableComponent(req.Component).
			SetNillableRouteName(req.RouteName).
			SetNillableQuery(req.Query).
			SetIsFrame(req.IsFrame).
			SetVisible(req.Visible).
			SetNillablePermissions(req.Permissions).
			SetStatus(req.Status).
			SetNillableParentID(req.ParentID).
			// 先清空再按请求重建 API 资源关联；请求为空即清除全部关联。
			ClearAPIResources()
		if len(req.ApiResources) > 0 {
			updateQuery = updateQuery.AddAPIResourceIDs(req.ApiResources...)
		}

		_, err := updateQuery.Save(txCtx)
		return err
	})
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrMenuNotFound
		}
		return nil, err
	}

	// Fetch updated menu with API resources
	return mr.GetMenuByID(ctx, id)
}

// GetChildrenMenus retrieves all direct children of a menu
func (mr *entMenuRepository) GetChildrenMenus(ctx context.Context, parentID string) ([]*domain.Menu, error) {
	menus, err := clientFromContext(ctx, mr.client).Menu.
		Query().
		Where(menu.ParentID(parentID)).
		Order(menu.BySequence()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Menu, 0, len(menus))
	for _, m := range menus {
		result = append(result, entMenuToDomain(m))
	}

	return result, nil
}

// DeleteMenu deletes a menu (only the specified menu, not children)
func (mr *entMenuRepository) DeleteMenu(ctx context.Context, id string) error {
	// Delete the menu directly without checking children.
	return WithAuthorizationTx(ctx, mr.client, func(txCtx context.Context, tx *ent.Tx) error {
		return tx.Menu.DeleteOneID(id).Exec(txCtx)
	})
}

func (mr *entMenuRepository) DeleteMenuTree(ctx context.Context, id string) error {
	return WithAuthorizationTx(ctx, mr.client, func(txCtx context.Context, tx *ent.Tx) error {
		return deleteMenuTree(txCtx, tx.Client(), id)
	})
}

func deleteMenuTree(ctx context.Context, client *ent.Client, id string) error {
	children, err := client.Menu.
		Query().
		Where(menu.ParentID(id)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to get children of menu %s: %w", id, err)
	}

	for _, child := range children {
		if err := deleteMenuTree(ctx, client, child.ID); err != nil {
			return fmt.Errorf("failed to delete child menu %s: %w", child.ID, err)
		}
	}

	if err := client.Menu.DeleteOneID(id).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete menu %s from database: %w", id, err)
	}
	return nil
}
