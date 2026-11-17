package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDictTypeCodeExists      = errors.New("dictionary type code already exists")
	ErrInvalidDictStatus       = errors.New("invalid dictionary status")
	ErrDictTypeHasItems        = errors.New("dictionary type has items, cannot delete")
	ErrDictTypeNotFound        = errors.New("dictionary type not found")
	ErrDictItemNotFound        = errors.New("dictionary item not found")
	ErrDictItemValueExists     = errors.New("dictionary item value already exists in this type")
	ErrDictItemLabelExists     = errors.New("dictionary item label already exists in this type")
	ErrDictItemDefaultConflict = errors.New("dictionary type already has a default item")
)

type DictType struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"` // 唯一标识
	Name      string    `json:"name"`
	Status    string    `json:"status"` // active, inactive
	Remark    string    `json:"remark,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DictItem struct {
	ID        string    `json:"id"`
	TypeID    string    `json:"type_id"`
	Label     string    `json:"label"`
	Value     string    `json:"value"`
	Sort      int       `json:"sort"`
	IsDefault bool      `json:"is_default"`
	Status    string    `json:"status"` // active, inactive
	Color     string    `json:"color,omitempty"`
	Remark    string    `json:"remark,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateDictTypeRequest struct {
	Code   string `json:"code" binding:"required"` // 唯一标识
	Name   string `json:"name" binding:"required"`
	Status string `json:"status,omitempty"` // active, inactive，默认active
	Remark string `json:"remark,omitempty"`
}

type UpdateDictTypeRequest struct {
	Code   *string `json:"code,omitempty"`
	Name   *string `json:"name,omitempty"`
	Status *string `json:"status,omitempty"`
	Remark *string `json:"remark,omitempty"`
}

type DictTypeQueryParams struct {
	Code    string `json:"code,omitempty" form:"code"`
	Name    string `json:"name,omitempty" form:"name"`
	Status  string `json:"status,omitempty" form:"status"`
	Keyword string `json:"keyword,omitempty" form:"keyword"` // 模糊搜索code和name
	QueryParams
}

type CreateDictItemRequest struct {
	TypeID    string `json:"type_id" binding:"required"`
	Label     string `json:"label" binding:"required"`
	Value     string `json:"value" binding:"required"`
	Sort      int    `json:"sort,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
	Status    string `json:"status,omitempty"` // active, inactive，默认active
	Color     string `json:"color,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

type UpdateDictItemRequest struct {
	Label     *string `json:"label,omitempty"`
	Value     *string `json:"value,omitempty"`
	Sort      *int    `json:"sort,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
	Status    *string `json:"status,omitempty"`
	Color     *string `json:"color,omitempty"`
	Remark    *string `json:"remark,omitempty"`
}

type DictItemQueryParams struct {
	TypeID   string `json:"type_id,omitempty" form:"type_id"`
	TypeCode string `json:"type_code,omitempty" form:"type_code"`
	Label    string `json:"label,omitempty" form:"label"`
	Value    string `json:"value,omitempty" form:"value"`
	Status   string `json:"status,omitempty" form:"status"`
	Keyword  string `json:"keyword,omitempty" form:"keyword"` // 模糊搜索label和value
	QueryParams
}

type DictTypePagedResult = PagedResult[*DictType]

type DictItemPagedResult = PagedResult[*DictItem]

type DictRepository interface {
	CreateType(ctx context.Context, dictType *DictType) error
	GetTypeByID(ctx context.Context, id string) (*DictType, error)
	GetTypeByCode(ctx context.Context, code string) (*DictType, error)
	FetchTypes(ctx context.Context, params DictTypeQueryParams) (*PagedResult[*DictType], error)
	UpdateType(ctx context.Context, id string, updates UpdateDictTypeRequest) error
	DeleteType(ctx context.Context, id string) error

	CreateItem(ctx context.Context, dictItem *DictItem) error
	GetItemByID(ctx context.Context, id string) (*DictItem, error)
	FetchItems(ctx context.Context, params DictItemQueryParams) (*PagedResult[*DictItem], error)
	UpdateItem(ctx context.Context, id string, updates UpdateDictItemRequest) error
	DeleteItem(ctx context.Context, id string) error

	GetItemsByTypeCode(ctx context.Context, typeCode string, status string) ([]*DictItem, error)
}

type DictUseCase interface {
	CreateDictType(ctx context.Context, request *CreateDictTypeRequest) (*DictType, error)
	GetDictTypeByID(ctx context.Context, id string) (*DictType, error)
	GetDictTypeByCode(ctx context.Context, code string) (*DictType, error)
	ListDictTypes(ctx context.Context, params DictTypeQueryParams) (*PagedResult[*DictType], error)
	UpdateDictType(ctx context.Context, id string, updates UpdateDictTypeRequest) error
	DeleteDictType(ctx context.Context, id string) error

	CreateDictItem(ctx context.Context, request *CreateDictItemRequest) (*DictItem, error)
	GetDictItemByID(ctx context.Context, id string) (*DictItem, error)
	ListDictItems(ctx context.Context, params DictItemQueryParams) (*PagedResult[*DictItem], error)
	UpdateDictItem(ctx context.Context, id string, updates UpdateDictItemRequest) error
	DeleteDictItem(ctx context.Context, id string) error

	GetActiveItemsByTypeCode(ctx context.Context, typeCode string) ([]*DictItem, error)
}
