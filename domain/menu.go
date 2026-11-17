package domain

import (
	"context"
	"errors"
	"time"
)

type Menu struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Sequence    int       `json:"sequence"`
	Type        string    `json:"type"` // menu/button
	Path        *string   `json:"path,omitempty"`
	Icon        string    `json:"icon"`
	Component   *string   `json:"component,omitempty"`
	RouteName   *string   `json:"route_name,omitempty"`
	Query       *string   `json:"query,omitempty"`
	IsFrame     bool      `json:"is_frame"`
	Visible     string    `json:"visible"` // show/hide
	Permissions *string   `json:"permissions,omitempty"`
	Status      string    `json:"status"` // active/inactive
	ParentID    *string   `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Children     []Menu   `json:"children,omitempty"`
	ApiResources []string `json:"apiResources,omitempty"`
}

type CreateMenuRequest struct {
	Name         string   `json:"name" binding:"required"`
	Sequence     int      `json:"sequence"`
	Type         string   `json:"type" binding:"required"` // menu/button
	Path         *string  `json:"path"`
	Icon         string   `json:"icon"`
	Component    *string  `json:"component"`
	RouteName    *string  `json:"route_name"`
	Query        *string  `json:"query"`
	IsFrame      bool     `json:"is_frame"`
	Visible      string   `json:"visible"` // show/hide
	Permissions  *string  `json:"permissions"`
	ApiResources []string `json:"apiResources"`
	Status       string   `json:"status"` // active/inactive
	ParentID     *string  `json:"parent_id"`
}

type UpdateMenuRequest struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Sequence     int      `json:"sequence"`
	Type         string   `json:"type"` // menu/button
	Path         *string  `json:"path"`
	Icon         string   `json:"icon"`
	Component    *string  `json:"component"`
	RouteName    *string  `json:"route_name"`
	Query        *string  `json:"query"`
	IsFrame      bool     `json:"is_frame"`
	Visible      string   `json:"visible"` // show/hide
	Permissions  *string  `json:"permissions"`
	ParentID     *string  `json:"parent_id"`
	Remark       *string  `json:"remark"`
	Properties   *string  `json:"properties"`
	Status       string   `json:"status"` // active/inactive
	ApiResources []string `json:"apiResources"`
}

type MenuTreeNode struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Sequence     int            `json:"sequence"`
	Type         string         `json:"type"`
	Path         *string        `json:"path,omitempty"`
	Icon         string         `json:"icon"`
	Component    *string        `json:"component,omitempty"`
	RouteName    *string        `json:"route_name,omitempty"`
	Query        *string        `json:"query,omitempty"`
	IsFrame      bool           `json:"is_frame"`
	Visible      string         `json:"visible"`
	Permissions  *string        `json:"permissions,omitempty"`
	Properties   *string        `json:"properties,omitempty"`
	Status       string         `json:"status"`
	ParentID     *string        `json:"parent_id,omitempty"`
	ApiResources []string       `json:"apiResources,omitempty"`
	Roles        []string       `json:"roles,omitempty"`
	Children     []MenuTreeNode `json:"children,omitempty"`
}

type AssignMenuRoleRequest struct {
	MenuID string   `json:"menu_id" binding:"required"`
	Roles  []string `json:"roles" binding:"required"`
}

type RemoveMenuRoleRequest struct {
	MenuID string   `json:"menu_id" binding:"required"`
	Roles  []string `json:"roles" binding:"required"`
}

// MenuQueryParams 不分页，返回全部匹配项。
type MenuQueryParams struct {
	Keyword string `form:"keyword"` // 模糊匹配 name/path
}

type MenuItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Icon          string `json:"icon"`
	Component     string `json:"component"`
	Sort          int    `json:"sort"`
	Visible       bool   `json:"visible"`
	RequiresAuth  bool   `json:"requiresAuth"`
	RequiresAdmin bool   `json:"requiresAdmin"`
}

const (
	MenuStatusActive   = StatusActive
	MenuStatusInactive = StatusInactive
)

const (
	MenuTypeMenu   = "menu"
	MenuTypeButton = "button"
)

var (
	ErrInvalidMenuType   = errors.New("invalid menu type")
	ErrInvalidMenuStatus = errors.New("invalid menu status")
	ErrInvalidHTTPMethod = errors.New("invalid HTTP method")
	ErrMenuNotFound      = errors.New("menu not found")
	ErrMenuHasChildren   = errors.New("cannot delete menu with children")
)

type MenuRepository interface {
	GetMenuTree(ctx context.Context) ([]MenuTreeNode, error)
	GetMenus(ctx context.Context, params MenuQueryParams) (*PagedResult[Menu], error)
	GetMenuByID(ctx context.Context, id string) (*Menu, error)
	GetMenuPermissionsByIDs(ctx context.Context, ids []string) ([]*Menu, error)
	GetChildrenMenus(ctx context.Context, parentID string) ([]*Menu, error)

	CreateMenu(ctx context.Context, menu *CreateMenuRequest) (*Menu, error)
	UpdateMenu(ctx context.Context, id string, menu *UpdateMenuRequest) (*Menu, error)
	DeleteMenu(ctx context.Context, id string) error
	DeleteMenuTree(ctx context.Context, id string) error
}

type MenuUseCase interface {
	GetMenuTree(ctx context.Context) ([]MenuTreeNode, error)
	GetMenus(ctx context.Context, params MenuQueryParams) (*PagedResult[Menu], error)
	GetMenuByID(ctx context.Context, id string) (*Menu, error)

	CreateMenu(ctx context.Context, menu *CreateMenuRequest) (*Menu, error)
	UpdateMenu(ctx context.Context, id string, menu *UpdateMenuRequest) (*Menu, error)
	DeleteMenu(ctx context.Context, id string) error
}
