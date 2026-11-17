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

func (mr *entMenuRepository) GetMenuTree(ctx context.Context) ([]domain.MenuTreeNode, error) {
	menus, err := clientFromContext(ctx, mr.client).Menu.
		Query().
		Where(menu.Status(domain.MenuStatusActive)).
		WithAPIResources().
		Order(menu.BySequence()).
		All(ctx)

	if err != nil {
		return nil, err
	}

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

	return buildTree(allNodes,
		func(n domain.MenuTreeNode) string { return n.ID },
		func(n domain.MenuTreeNode) *string { return n.ParentID },
		func(n *domain.MenuTreeNode, children []domain.MenuTreeNode) { n.Children = children },
	), nil
}

// GetMenus 返回全部匹配项，不分页。
func (mr *entMenuRepository) GetMenus(ctx context.Context, params domain.MenuQueryParams) (*domain.PagedResult[domain.Menu], error) {
	query := clientFromContext(ctx, mr.client).Menu.Query()

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

	result := make([]domain.Menu, 0, len(menus))
	for _, m := range menus {
		result = append(result, *entMenuToDomain(m))
	}

	return domain.NewPagedResult(result, len(result), 1, max(len(result), 1)), nil
}

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

	return mr.GetMenuByID(ctx, createdID)
}

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
			// 空数组清除全部 API 资源关联。
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

	return mr.GetMenuByID(ctx, id)
}

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

// DeleteMenu 仅删除指定菜单，子菜单由调用方处理。
func (mr *entMenuRepository) DeleteMenu(ctx context.Context, id string) error {
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
