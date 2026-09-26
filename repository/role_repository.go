package repository

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/role"
	"shadmin/internal/constants"
	"time"
)

type entRoleRepository struct {
	client *ent.Client
}

func NewRoleRepository(client *ent.Client) domain.RoleRepository {
	return &entRoleRepository{
		client: client,
	}
}

// convertEntRoleToDomain converts an ent Role to domain Role
func (rr *entRoleRepository) convertEntRoleToDomain(entRole *ent.Role) *domain.Role {
	if entRole == nil {
		return nil
	}

	domainRole := &domain.Role{
		ID:        entRole.ID,
		Name:      entRole.Name,
		IsSystem:  entRole.IsSystem,
		Sequence:  entRole.Sequence,
		Status:    entRole.Status,
		CreatedAt: entRole.CreatedAt,
		UpdatedAt: entRole.UpdatedAt,
	}

	// Extract menu IDs from edges if available
	if entRole.Edges.Menus != nil {
		menuIDs := make([]string, len(entRole.Edges.Menus))
		for i, menu := range entRole.Edges.Menus {
			menuIDs[i] = menu.ID
		}
		domainRole.MenusIds = menuIDs
	}

	return domainRole
}

func (rr *entRoleRepository) Create(c context.Context, role *domain.Role) error {
	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now

	var created *ent.Role
	err := WithAuthorizationTx(c, rr.client, func(txCtx context.Context, tx *ent.Tx) error {
		var err error
		created, err = tx.Role.Create().
			SetName(role.Name).
			SetSequence(role.Sequence).
			SetStatus(role.Status).
			AddMenuIDs(role.MenusIds...).
			SetCreatedAt(role.CreatedAt).
			SetUpdatedAt(role.UpdatedAt).
			Save(txCtx)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	role.ID = created.ID
	return nil
}

func (rr *entRoleRepository) Fetch(c context.Context) ([]*domain.Role, error) {
	entRoles, err := rr.client.Role.Query().
		WithMenus().
		Order(ent.Asc(role.FieldSequence)).
		All(c)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}

	roles := make([]*domain.Role, len(entRoles))
	for i, entRole := range entRoles {
		roles[i] = rr.convertEntRoleToDomain(entRole)
	}

	return roles, nil
}

func (rr *entRoleRepository) GetByID(c context.Context, id string) (*domain.Role, error) {
	entRole, err := rr.client.Role.Query().
		Where(role.ID(id)).
		WithMenus().
		Only(c)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by ID: %w", err)
	}

	return rr.convertEntRoleToDomain(entRole), nil
}

func (rr *entRoleRepository) GetByName(c context.Context, name string) (*domain.Role, error) {
	entRole, err := rr.client.Role.Query().
		Where(role.Name(name)).
		WithMenus().
		Only(c)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}
	return rr.convertEntRoleToDomain(entRole), nil
}

// GetByIDs 批量按 ID 查询角色（含菜单 ID），把逐条 GetByID 的 N+1 回查收敛为单次查询。
func (rr *entRoleRepository) GetByIDs(c context.Context, ids []string) ([]*domain.Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	entRoles, err := rr.client.Role.Query().
		Where(role.IDIn(ids...)).
		WithMenus().
		All(c)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles by IDs: %w", err)
	}

	roles := make([]*domain.Role, len(entRoles))
	for i, entRole := range entRoles {
		roles[i] = rr.convertEntRoleToDomain(entRole)
	}
	return roles, nil
}

// ExistsByName 仅判定角色名称是否被占用
func (rr *entRoleRepository) ExistsByName(c context.Context, name string) (bool, error) {
	exists, err := rr.client.Role.Query().
		Where(role.Name(name)).
		Exist(c)
	if err != nil {
		return false, fmt.Errorf("failed to check role name: %w", err)
	}

	return exists, nil
}

// Update 更新角色信息
// 🔒 安全: IsSystem 字段是 Immutable 的，Ent 层面禁止更新
func (rr *entRoleRepository) Update(c context.Context, role *domain.Role) error {
	role.UpdatedAt = time.Now()

	err := WithAuthorizationTx(c, rr.client, func(txCtx context.Context, tx *ent.Tx) error {
		_, err := tx.Role.UpdateOneID(role.ID).
			SetName(role.Name).
			SetSequence(role.Sequence).
			SetStatus(role.Status).
			SetUpdatedAt(role.UpdatedAt).
			ClearMenus().
			AddMenuIDs(role.MenusIds...).
			Save(txCtx)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	return nil
}

func (rr *entRoleRepository) Delete(c context.Context, id string) error {
	err := WithAuthorizationTx(c, rr.client, func(txCtx context.Context, tx *ent.Tx) error {
		return tx.Role.DeleteOneID(id).Exec(txCtx)
	})
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrRoleNotFound
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}

	return nil
}

func (rr *entRoleRepository) DeleteIfUnused(c context.Context, id, name string) error {
	tx, err := rr.client.Tx(c)
	if err != nil {
		return fmt.Errorf("failed to begin role deletion transaction: %w", err)
	}
	defer tx.Rollback()
	if err := BumpAuthorizationGeneration(c, tx); err != nil {
		return err
	}

	userCount, err := tx.Role.
		Query().
		Where(role.IDEQ(id)).
		QueryUsers().
		Count(c)
	if err != nil {
		return fmt.Errorf("failed to check role usage: %w", err)
	}
	if userCount > 0 {
		return fmt.Errorf("%w: role %s still assigned to %d users", domain.ErrRoleInUse, name, userCount)
	}

	if err := tx.Role.DeleteOneID(id).Exec(c); err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrRoleNotFound
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit role deletion transaction: %w", err)
	}
	return nil
}

// GetAllRoleNames returns active role names for downstream consumers.
func (rr *entRoleRepository) GetAllRoleNames(c context.Context) ([]string, error) {
	entRoles, err := rr.client.Role.Query().
		Where(role.Status(constants.StatusActive)).
		Select(role.FieldName).
		All(c)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all role names: %w", err)
	}

	names := make([]string, len(entRoles))
	for i, entRole := range entRoles {
		names[i] = entRole.Name
	}

	return names, nil
}
