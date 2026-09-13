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

// GetMenuTree retrieves menu tree structure
func (mr *entMenuRepository) GetMenuTree(ctx context.Context) ([]domain.MenuTreeNode, error) {
	// Query all active menus, ordered by sequence, include API resources
	menus, err := mr.client.Menu.
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

// GetMenus retrieves paginated menus with filtering
func (mr *entMenuRepository) GetMenus(ctx context.Context, params domain.MenuQueryParams) (*domain.PagedResult[domain.Menu], error) {
	query := mr.client.Menu.Query()

	// keyword：模糊匹配 name / path
	kw := strings.TrimSpace(params.Keyword)
	if kw != "" {
		query = query.Where(menu.Or(
			menu.NameContains(kw),
			menu.PathContains(kw),
		))
	}

	// Get total count
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	// 不需要limit限制
	offset, _ := params.Paginate()
	menus, err := query.
		WithAPIResources().
		Order(menu.BySequence(), menu.ByCreatedAt()).
		Offset(offset).
		All(ctx)

	if err != nil {
		return nil, err
	}

	// Convert to domain menus
	var result []domain.Menu
	for _, m := range menus {
		// Extract API resource IDs
		var apiResourceIDs []string
		for _, apiResource := range m.Edges.APIResources {
			apiResourceIDs = append(apiResourceIDs, apiResource.ID)
		}

		result = append(result, domain.Menu{
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
		})
	}

	return domain.NewPagedResult(result, total, params.Page, params.PageSize), nil
}

// GetMenuByID retrieves a menu by its ID
func (mr *entMenuRepository) GetMenuByID(ctx context.Context, id string) (*domain.Menu, error) {
	m, err := mr.client.Menu.
		Query().
		Where(menu.ID(id)).
		WithAPIResources().
		First(ctx)

	if err != nil {
		return nil, err
	}

	// Extract API resource IDs
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
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		ApiResources: apiResourceIDs,
	}, nil
}

// CreateMenu creates a new menu
func (mr *entMenuRepository) CreateMenu(ctx context.Context, req *domain.CreateMenuRequest) (*domain.Menu, error) {
	tx, err := mr.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

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

	created, err := createQuery.Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Fetch created menu with API resources
	return mr.GetMenuByID(ctx, created.ID)
}

// UpdateMenu updates an existing menu
func (mr *entMenuRepository) UpdateMenu(ctx context.Context, id string, req *domain.UpdateMenuRequest) (*domain.Menu, error) {
	tx, err := mr.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

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
		SetNillableParentID(req.ParentID)

	// Handle API resource associations
	if len(req.ApiResources) > 0 {
		// Clear existing API resource associations
		updateQuery = updateQuery.ClearAPIResources()

		// Add new API resource associations
		updateQuery = updateQuery.AddAPIResourceIDs(req.ApiResources...)
	} else {
		// If no API resources provided, clear all associations
		updateQuery = updateQuery.ClearAPIResources()
	}

	updated, err := updateQuery.Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Fetch updated menu with API resources
	return mr.GetMenuByID(ctx, updated.ID)
}

// GetChildrenMenus retrieves all direct children of a menu
func (mr *entMenuRepository) GetChildrenMenus(ctx context.Context, parentID string) ([]*domain.Menu, error) {
	menus, err := mr.client.Menu.
		Query().
		Where(menu.ParentID(parentID)).
		Order(menu.BySequence()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	var result []*domain.Menu
	for _, m := range menus {
		result = append(result, &domain.Menu{
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
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		})
	}

	return result, nil
}

// DeleteMenu deletes a menu (only the specified menu, not children)
func (mr *entMenuRepository) DeleteMenu(ctx context.Context, id string) error {
	// Delete the menu directly without checking children
	// The recursive deletion logic will be handled in the UseCase layer
	return mr.client.Menu.
		DeleteOneID(id).
		Exec(ctx)
}

func (mr *entMenuRepository) DeleteMenuTree(ctx context.Context, id string) error {
	tx, err := mr.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin menu deletion transaction: %w", err)
	}
	defer tx.Rollback()

	if err := deleteMenuTree(ctx, tx.Client(), id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit menu deletion transaction: %w", err)
	}
	return nil
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
