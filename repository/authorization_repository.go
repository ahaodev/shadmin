package repository

import (
	"context"
	"fmt"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/role"
	"shadmin/ent/user"
)

type entAuthorizationRepository struct {
	client *ent.Client
}

// NewAuthorizationRepository 构造 Casbin 快照的权限数据源（ent 实现）。
func NewAuthorizationRepository(client *ent.Client) domain.AuthorizationSource {
	return &entAuthorizationRepository{client: client}
}

func (r *entAuthorizationRepository) Generation(ctx context.Context) (int64, error) {
	return CurrentAuthorizationGeneration(ctx, r.client)
}

func (r *entAuthorizationRepository) ActiveUserRoles(ctx context.Context) ([]domain.UserRoleBinding, error) {
	users, err := r.client.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query active user-role relationships: %w", err)
	}

	bindings := make([]domain.UserRoleBinding, 0, len(users))
	for _, u := range users {
		b := domain.UserRoleBinding{UserID: u.ID}
		for _, r := range u.Edges.Roles {
			b.RoleIDs = append(b.RoleIDs, r.ID)
		}
		bindings = append(bindings, b)
	}
	return bindings, nil
}

func (r *entAuthorizationRepository) ActiveRoles(ctx context.Context) ([]domain.AuthorizedRole, error) {
	roles, err := r.client.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.WithAPIResources()
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query active role permissions: %w", err)
	}

	result := make([]domain.AuthorizedRole, 0, len(roles))
	for _, ro := range roles {
		ar := domain.AuthorizedRole{ID: ro.ID, Name: ro.Name}
		for _, m := range ro.Edges.Menus {
			for _, res := range m.Edges.APIResources {
				ar.Resources = append(ar.Resources, domain.AuthorizedResource{
					Method:   res.Method,
					Path:     res.Path,
					IsPublic: res.IsPublic,
				})
			}
		}
		result = append(result, ar)
	}
	return result, nil
}
