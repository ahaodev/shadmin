package domain

import "context"

type UserResources struct {
	Menus       []MenuTreeNode `json:"menus"`
	Permissions []string       `json:"permissions"`
	Roles       []string       `json:"roles"`
	IsAdmin     bool           `json:"is_admin"`
}

type ResourceUseCase interface {
	GetUserResources(ctx context.Context, userID string, isAdmin bool) (*UserResources, error)
}
