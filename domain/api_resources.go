package domain

import (
	"context"
	"strings"
	"time"
)

type ApiResource struct {
	ID        string    `json:"id"` // format: "method:path"
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Handler   string    `json:"handler"`
	Module    string    `json:"module"`
	IsPublic  bool      `json:"is_public"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ApiResourceKey struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// GenerateApiResourceID 返回稳定的 METHOD:path 标识。
func GenerateApiResourceID(method, path string) string {
	return method + ":" + path
}

func ParseApiResourceID(id string) (method, path string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

type ApiResourcePagedResult = PagedResult[*ApiResource]

type ApiResourceQueryParams struct {
	QueryParams
	Method   string `json:"method"`
	Module   string `json:"module"`
	Status   string `json:"status"`
	IsPublic *bool  `json:"is_public"`
	Keyword  string `json:"keyword"`
	Path     string `json:"path"`
}

type ApiResourceUseCase interface {
	FetchPaged(ctx context.Context, params ApiResourceQueryParams) (*ApiResourcePagedResult, error)
}

type ApiResourceRepository interface {
	FetchPaged(ctx context.Context, params ApiResourceQueryParams) (*ApiResourcePagedResult, error)
}
