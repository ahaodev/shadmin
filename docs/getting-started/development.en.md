# Development Guide

This document walks you through adding a complete **"Project Management"** CRUD module as an example — from backend API to frontend page — demonstrating how to develop new features in Shadmin.

## Prerequisites

- Completed [Quick Start](./quickstart.en.md) and the project is running
- Read [Architecture Overview](./architecture.en.md) and understand the layered design
- Familiar with Go and React/TypeScript basics

---

## Backend Development

### Step 1: Define the Domain Layer

The Domain layer is the contract layer for the entire module, defining entities, DTOs, interfaces, and error constants.

Create `domain/project.go`:

```go
package domain

import (
	"context"
	"errors"
	"time"
)

// ========== Entity ==========

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active, archived
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== Request DTOs ==========

// Create request: required fields use binding:"required"
type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// Update request: optional fields use pointers to distinguish "not provided" from "null"
type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// Query params: embed QueryParams to get pagination support
type ProjectQueryParams struct {
	QueryParams
	Name   string `form:"name"`
	Code   string `form:"code"`
	Status string `form:"status"`
	Search string `form:"search"`
}

// ========== Interfaces ==========

type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
	Fetch(ctx context.Context, params ProjectQueryParams) (*PagedResult[Project], error)
	Update(ctx context.Context, id string, req *UpdateProjectRequest) error
	Delete(ctx context.Context, id string) error
}

type ProjectUseCase interface {
	Create(ctx context.Context, req *CreateProjectRequest) (*Project, error)
	GetByID(ctx context.Context, id string) (*Project, error)
	List(ctx context.Context, params ProjectQueryParams) (*PagedResult[Project], error)
	Update(ctx context.Context, id string, req *UpdateProjectRequest) error
	Delete(ctx context.Context, id string) error
}

// ========== Error Constants ==========

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrProjectCodeExists = errors.New("project code already exists")
)
```

**Key Points:**
- `QueryParams` is a built-in pagination base struct (Page, PageSize, SortBy, Order)
- `PagedResult[T]` is a generic paginated response
- Repository interface uses `Fetch` for paginated queries, `GetByID` for single record queries
- Update requests use pointer types to only update non-nil fields

### Step 2: Create the Ent Schema

Ent is the ORM used by Shadmin, defining database table structures through Go code.

Create `ent/schema/project.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/rs/xid"
)

type Project struct {
	ent.Schema
}

func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(20).
			NotEmpty().
			Unique().
			Immutable().
			DefaultFunc(func() string {
				return xid.New().String()
			}),
		field.String("name").
			NotEmpty().
			Comment("Project name"),
		field.String("code").
			NotEmpty().
			Comment("Project code"),
		field.String("description").
			Default("").
			Comment("Project description"),
		field.Enum("status").
			Values("active", "archived").
			Default("active").
			Comment("Status"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("Created at"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Updated at"),
	}
}

func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("status"),
		index.Fields("name"),
	}
}
```

Then generate the ORM code:

```bash
go generate ./ent
```

This generates `project.go`, `project_create.go`, `project_query.go`, etc. under `ent/`.

**Key Points:**
- IDs use `xid.New().String()` — globally unique and sortable
- `created_at` is set to `Immutable()` — cannot be modified after creation
- `updated_at` uses `UpdateDefault(time.Now)` for automatic updates
- Enum fields use `field.Enum()` instead of strings for database-level constraints

### Step 3: Implement the Repository

The Repository handles data access and implements the Domain interfaces.

Create `repository/project_repository.go`:

```go
package repository

import (
	"context"
	"math"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/project"
)

type projectRepository struct {
	client *ent.Client
}

func NewProjectRepository(client *ent.Client) domain.ProjectRepository {
	return &projectRepository{client: client}
}

func (r *projectRepository) Create(ctx context.Context, p *domain.Project) error {
	// Check code uniqueness
	exists, _ := r.client.Project.Query().
		Where(project.Code(p.Code)).
		Exist(ctx)
	if exists {
		return domain.ErrProjectCodeExists
	}

	result, err := r.client.Project.Create().
		SetName(p.Name).
		SetCode(p.Code).
		SetDescription(p.Description).
		SetStatus(project.Status(p.Status)).
		Save(ctx)
	if err != nil {
		return err
	}

	p.ID = result.ID
	p.CreatedAt = result.CreatedAt
	p.UpdatedAt = result.UpdatedAt
	return nil
}

func (r *projectRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	result, err := r.client.Project.Query().
		Where(project.ID(id)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, err
	}
	return r.toDomain(result), nil
}

func (r *projectRepository) Fetch(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[domain.Project], error) {
	query := r.client.Project.Query()

	// Apply filters
	if params.Name != "" {
		query = query.Where(project.NameContains(params.Name))
	}
	if params.Code != "" {
		query = query.Where(project.CodeContains(params.Code))
	}
	if params.Status != "" {
		query = query.Where(project.StatusEQ(project.Status(params.Status)))
	}
	if params.Search != "" {
		query = query.Where(
			project.Or(
				project.NameContains(params.Search),
				project.CodeContains(params.Search),
			),
		)
	}

	// Get total count (clone the query)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	// Sorting
	if params.SortBy != "" {
		if params.Order == "desc" {
			query = query.Order(ent.Desc(params.SortBy))
		} else {
			query = query.Order(ent.Asc(params.SortBy))
		}
	} else {
		query = query.Order(ent.Desc("created_at"))
	}

	// Pagination
	offset := (params.Page - 1) * params.PageSize
	results, err := query.Offset(offset).Limit(params.PageSize).All(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to domain entities
	list := make([]domain.Project, 0, len(results))
	for _, item := range results {
		list = append(list, *r.toDomain(item))
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &domain.PagedResult[domain.Project]{
		List:       list,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *projectRepository) Update(ctx context.Context, id string, req *domain.UpdateProjectRequest) error {
	// Check record exists
	_, err := r.client.Project.Query().
		Where(project.ID(id)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrProjectNotFound
		}
		return err
	}

	// If changing code, check new code uniqueness
	if req.Code != nil {
		exists, _ := r.client.Project.Query().
			Where(project.Code(*req.Code), project.IDNEQ(id)).
			Exist(ctx)
		if exists {
			return domain.ErrProjectCodeExists
		}
	}

	update := r.client.Project.UpdateOneID(id)
	if req.Name != nil {
		update = update.SetName(*req.Name)
	}
	if req.Code != nil {
		update = update.SetCode(*req.Code)
	}
	if req.Description != nil {
		update = update.SetDescription(*req.Description)
	}
	if req.Status != nil {
		update = update.SetStatus(project.Status(*req.Status))
	}

	return update.Exec(ctx)
}

func (r *projectRepository) Delete(ctx context.Context, id string) error {
	err := r.client.Project.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrProjectNotFound
		}
		return err
	}
	return nil
}

// Ent entity → Domain entity conversion
func (r *projectRepository) toDomain(e *ent.Project) *domain.Project {
	return &domain.Project{
		ID:          e.ID,
		Name:        e.Name,
		Code:        e.Code,
		Description: e.Description,
		Status:      string(e.Status),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
```

**Key Points:**
- Constructor returns the interface type (`domain.ProjectRepository`), following dependency inversion
- Paginated queries use `Clone()` to get the total count, then apply sorting + pagination
- Updates only set non-nil fields
- Use `ent.IsNotFound()` to check for missing records
- `toDomain` helper method converts Ent entities to Domain entities

### Step 4: Implement the Usecase

The Usecase orchestrates business logic and bridges the Controller and Repository.

Create `usecase/project_usecase.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"time"
)

type projectUsecase struct {
	client            *ent.Client
	projectRepository domain.ProjectRepository
	contextTimeout    time.Duration
}

func NewProjectUsecase(
	client *ent.Client,
	projectRepository domain.ProjectRepository,
	timeout time.Duration,
) domain.ProjectUseCase {
	return &projectUsecase{
		client:            client,
		projectRepository: projectRepository,
		contextTimeout:    timeout,
	}
}

func (u *projectUsecase) Create(ctx context.Context, req *domain.CreateProjectRequest) (*domain.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// Set defaults
	if req.Status == "" {
		req.Status = "active"
	}

	// Validate enum values
	if req.Status != "active" && req.Status != "archived" {
		return nil, fmt.Errorf("invalid status: must be 'active' or 'archived'")
	}

	p := &domain.Project{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Status:      req.Status,
	}

	if err := u.projectRepository.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *projectUsecase) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.projectRepository.GetByID(ctx, id)
}

func (u *projectUsecase) List(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[domain.Project], error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	domain.ValidateQueryParams(&params.QueryParams)
	return u.projectRepository.Fetch(ctx, params)
}

func (u *projectUsecase) Update(ctx context.Context, id string, req *domain.UpdateProjectRequest) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// Validate enum values
	if req.Status != nil && *req.Status != "active" && *req.Status != "archived" {
		return fmt.Errorf("invalid status: must be 'active' or 'archived'")
	}

	return u.projectRepository.Update(ctx, id, req)
}

func (u *projectUsecase) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.projectRepository.Delete(ctx, id)
}
```

**Key Points:**
- Every method wraps the context with `context.WithTimeout` to prevent blocking
- `ValidateQueryParams` validates and corrects pagination params (Page < 1 → 1, PageSize over limit → truncated)
- Usecase never touches the database directly — it only calls Repository interfaces
- Business validation (enum values, defaults) belongs in the Usecase layer

### Step 5: Write the Controller

The Controller is the HTTP handler, responsible for request parsing, response formatting, and Swagger documentation.

Create `api/controller/project_controller.go`:

```go
package controller

import (
	"net/http"
	"shadmin/domain"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProjectController struct {
	ProjectUseCase domain.ProjectUseCase
}

// @Summary     List projects
// @Description Get paginated project list with search and filters
// @Tags        Project Management
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       page      query    int    false  "Page number"
// @Param       page_size query    int    false  "Page size"
// @Param       name      query    string false  "Project name"
// @Param       code      query    string false  "Project code"
// @Param       status    query    string false  "Status"
// @Param       search    query    string false  "Search keyword"
// @Param       sort_by   query    string false  "Sort field"
// @Param       order     query    string false  "Sort direction"
// @Success     200       {object} domain.Response
// @Failure     500       {object} domain.Response
// @Router      /system/projects [get]
func (pc *ProjectController) List(c *gin.Context) {
	var params domain.ProjectQueryParams

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			params.Page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			params.PageSize = v
		}
	}
	params.Name = c.Query("name")
	params.Code = c.Query("code")
	params.Status = c.Query("status")
	params.Search = c.Query("search")
	params.SortBy = c.Query("sort_by")
	params.Order = c.Query("order")

	result, err := pc.ProjectUseCase.List(c, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// @Summary     Get project details
// @Description Get project by ID
// @Tags        Project Management
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "Project ID"
// @Success     200 {object} domain.Response
// @Failure     404 {object} domain.Response
// @Router      /system/projects/{id} [get]
func (pc *ProjectController) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := pc.ProjectUseCase.GetByID(c, id)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// @Summary     Create project
// @Description Create a new project
// @Tags        Project Management
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body     domain.CreateProjectRequest true "Create project request"
// @Success     201  {object} domain.Response
// @Failure     400  {object} domain.Response
// @Failure     409  {object} domain.Response
// @Router      /system/projects [post]
func (pc *ProjectController) Create(c *gin.Context) {
	var req domain.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	result, err := pc.ProjectUseCase.Create(c, &req)
	if err != nil {
		if err == domain.ErrProjectCodeExists {
			c.JSON(http.StatusConflict, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, domain.RespSuccess(result))
}

// @Summary     Update project
// @Description Update project information
// @Tags        Project Management
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path     string                     true "Project ID"
// @Param       body body     domain.UpdateProjectRequest true "Update project request"
// @Success     200  {object} domain.Response
// @Failure     400  {object} domain.Response
// @Failure     404  {object} domain.Response
// @Failure     409  {object} domain.Response
// @Router      /system/projects/{id} [put]
func (pc *ProjectController) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	err := pc.ProjectUseCase.Update(c, id, &req)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		if err == domain.ErrProjectCodeExists {
			c.JSON(http.StatusConflict, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(nil))
}

// @Summary     Delete project
// @Description Delete a project
// @Tags        Project Management
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "Project ID"
// @Success     200 {object} domain.Response
// @Failure     404 {object} domain.Response
// @Router      /system/projects/{id} [delete]
func (pc *ProjectController) Delete(c *gin.Context) {
	id := c.Param("id")

	err := pc.ProjectUseCase.Delete(c, id)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(nil))
}
```

**Key Points:**
- Query parameters are manually parsed (`c.Query` + `strconv.Atoi`), parse failures are silently ignored
- Body parameters use `c.ShouldBindJSON`, Gin auto-validates `binding:"required"` tags
- Path parameters use `c.Param("id")`
- Responses use `domain.RespSuccess(data)` / `domain.RespError(msg)` for a unified format
- Domain errors are mapped to corresponding HTTP status codes (404, 409, etc.)
- Swagger annotations are placed above each method, `@Security BearerAuth` indicates authentication required

### Step 6: Register Routes

Create `api/route/project_routes.go`:

```go
package route

import (
	"shadmin/api/middleware"

	"github.com/gin-gonic/gin"
)

func (pr *ProtectedRoutes) setupProjectManagement(
	systemGroup *gin.RouterGroup,
	casbinMiddleware *middleware.CasbinMiddleware,
) {
	projectGroup := systemGroup.Group("/projects")
	projectGroup.Use(casbinMiddleware.CheckAPIPermission())
	projectController := pr.factory.CreateProjectController()

	projectGroup.GET("", projectController.List)
	projectGroup.POST("", projectController.Create)
	projectGroup.GET("/:id", projectController.GetByID)
	projectGroup.PUT("/:id", projectController.Update)
	projectGroup.DELETE("/:id", projectController.Delete)
}
```

Then call it in the `SetupSystemRoutes` method in `api/route/system_routes.go`:

```go
func (pr *ProtectedRoutes) SetupSystemRoutes(...) {
    systemGroup := ...
    // Existing routes
    pr.setupUserManagement(systemGroup, casbinMiddleware)
    pr.setupRoleManagement(systemGroup, casbinMiddleware)
    // ...

    // Add new module
    pr.setupProjectManagement(systemGroup, casbinMiddleware)
}
```

**Key Points:**
- Each module has its own route file, with methods on `ProtectedRoutes`
- `casbinMiddleware.CheckAPIPermission()` automatically checks API-level permissions
- RESTful style: GET list, POST create, GET/:id detail, PUT/:id update, DELETE/:id delete

### Step 7: Wire Up the Factory

Add the constructor method in `api/route/factory.go`:

```go
func (f *ControllerFactory) CreateProjectController() *controller.ProjectController {
	projectRepository := repository.NewProjectRepository(f.db)
	projectUseCase := usecase.NewProjectUsecase(f.db, projectRepository, f.timeout)
	return &controller.ProjectController{ProjectUseCase: projectUseCase}
}
```

The factory assembles the dependency chain: `Repository → Usecase → Controller`. This is Shadmin's manual DI approach — no DI framework is used.

### Step 8: Generate Swagger Documentation

```bash
# Install swag (if not installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g main.go --output ./docs

# After starting the project, access the Swagger UI
# http://localhost:55667/swagger/index.html
```

Backend development is now complete. Start the project and test APIs using Swagger or curl:

```bash
# Login to get a token
curl -X POST http://localhost:55667/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# Create a project (replace YOUR_TOKEN)
curl -X POST http://localhost:55667/api/v1/system/projects \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Project","code":"test-001"}'
```

---

## Frontend Development

### Step 1: Define Types and API

#### Define Types

Create `web/src/types/project.ts`:

```typescript
// Status type
export type ProjectStatus = 'active' | 'archived'

// Entity interface
export interface Project {
  id: string
  name: string
  code: string
  description: string
  status: ProjectStatus
  created_at: Date
  updated_at: Date
}

// Create request
export interface CreateProjectRequest {
  name: string
  code: string
  description?: string
  status?: ProjectStatus
}

// Update request
export interface UpdateProjectRequest {
  name?: string
  code?: string
  description?: string
  status?: ProjectStatus
}

// Query parameters
export interface ProjectQueryParams {
  page?: number
  page_size?: number
  name?: string
  code?: string
  status?: ProjectStatus
  search?: string
  sort_by?: string
  order?: 'asc' | 'desc'
}

// Paginated result
export interface ProjectPagedResult {
  list: Project[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
```

#### Define API Service

Create `web/src/services/projectApi.ts`:

```typescript
import { apiClient } from './config'
import type {
  Project,
  CreateProjectRequest,
  UpdateProjectRequest,
  ProjectQueryParams,
  ProjectPagedResult,
} from '@/types/project'

// Date field parsing
const parseProject = (p: any): Project => ({
  ...p,
  created_at: new Date(p.created_at),
  updated_at: new Date(p.updated_at),
})

// GET /system/projects
export const getProjects = async (params?: ProjectQueryParams): Promise<ProjectPagedResult> => {
  const searchParams = new URLSearchParams()
  if (params?.page) searchParams.append('page', params.page.toString())
  if (params?.page_size) searchParams.append('page_size', params.page_size.toString())
  if (params?.name) searchParams.append('name', params.name)
  if (params?.code) searchParams.append('code', params.code)
  if (params?.status) searchParams.append('status', params.status)
  if (params?.search) searchParams.append('search', params.search)
  if (params?.sort_by) searchParams.append('sort_by', params.sort_by)
  if (params?.order) searchParams.append('order', params.order)

  const response = await apiClient.get(`/api/v1/system/projects?${searchParams}`)
  const data = response.data.data as ProjectPagedResult
  return {
    ...data,
    list: (data.list || []).map(parseProject),
  }
}

// GET /system/projects/:id
export const getProject = async (id: string): Promise<Project> => {
  const response = await apiClient.get(`/api/v1/system/projects/${id}`)
  return parseProject(response.data.data)
}

// POST /system/projects
export const createProject = async (data: CreateProjectRequest): Promise<Project> => {
  const response = await apiClient.post('/api/v1/system/projects', data)
  return parseProject(response.data.data)
}

// PUT /system/projects/:id
export const updateProject = async (id: string, data: UpdateProjectRequest): Promise<void> => {
  await apiClient.put(`/api/v1/system/projects/${id}`, data)
}

// DELETE /system/projects/:id
export const deleteProject = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/projects/${id}`)
}
```

**Key Points:**
- `apiClient` is imported from `./config`, pre-configured with baseURL and JWT interceptor
- Response data path is `response.data.data` (outer `.data` is Axios, inner `.data` is `domain.Response.Data`)
- Date fields need manual `new Date()` conversion

### Step 2: Create the Feature Module

Create the project module directory structure under `web/src/features/system/`:

```
web/src/features/system/projects/
├── components/
│   ├── projects-provider.tsx     # Context Provider (state management)
│   ├── projects-columns.tsx      # Table column definitions
│   ├── projects-table.tsx        # Data table
│   ├── projects-dialogs.tsx      # Dialog aggregation
│   ├── projects-primary-buttons.tsx  # Header action buttons
│   └── project-form-dialog.tsx   # Create/edit form dialog
├── data/
│   └── schema.ts                 # Zod validation schema
├── hooks/
│   └── use-projects.ts           # TanStack Query hooks
└── index.tsx                     # Page entry
```

#### Context Provider

Create `web/src/features/system/projects/components/projects-provider.tsx`:

```tsx
import { createContext, useContext, useMemo, useState, type ReactNode, type Dispatch, type SetStateAction } from 'react'
import type { Project } from '@/types/project'

interface ProjectsContext {
  currentRow: Project | null
  setCurrentRow: Dispatch<SetStateAction<Project | null>>
  showCreateDialog: boolean
  setShowCreateDialog: Dispatch<SetStateAction<boolean>>
  showEditDialog: boolean
  setShowEditDialog: Dispatch<SetStateAction<boolean>>
  showDeleteDialog: boolean
  setShowDeleteDialog: Dispatch<SetStateAction<boolean>>
}

const ProjectsContext = createContext<ProjectsContext | null>(null)

export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [currentRow, setCurrentRow] = useState<Project | null>(null)
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [showEditDialog, setShowEditDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const value: ProjectsContext = useMemo(() => ({
    currentRow, setCurrentRow,
    showCreateDialog, setShowCreateDialog,
    showEditDialog, setShowEditDialog,
    showDeleteDialog, setShowDeleteDialog,
  }), [currentRow, showCreateDialog, showEditDialog, showDeleteDialog])

  return (
    <ProjectsContext.Provider value={value}>
      {children}
    </ProjectsContext.Provider>
  )
}

export const useProjects = () => {
  const ctx = useContext(ProjectsContext)
  if (!ctx) throw new Error('useProjects must be used within <ProjectsProvider>')
  return ctx
}
```

**Key Points:**
- Each feature module uses Context to manage dialog states and current row data
- `useMemo` optimizes re-renders
- Custom hook `useProjects()` wraps Context consumption

### Step 3: Write Hooks

Create `web/src/features/system/projects/hooks/use-projects.ts`:

```typescript
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { ProjectQueryParams } from '@/types/project'
import { deleteProject, getProjects } from '@/services/projectApi'

const PROJECTS_QUERY_KEY = 'projects'

// List query
export function useProjectList(params?: ProjectQueryParams) {
  return useQuery({
    queryKey: [PROJECTS_QUERY_KEY, params],
    queryFn: () => getProjects(params),
    staleTime: 5 * 60 * 1000,
  })
}

// Delete mutation
export function useDeleteProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [PROJECTS_QUERY_KEY] })
      toast.success('Deleted successfully')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || 'Delete failed')
    },
  })
}
```

**Key Points:**
- Query key uses a string constant + params object — TanStack Query handles caching and invalidation automatically
- `staleTime` controls cache duration
- Mutations call `invalidateQueries` on success to refresh the list
- Error messages are extracted from the backend response's `msg` field

### Step 4: Implement the List Page

Create `web/src/features/system/projects/index.tsx`:

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Skeleton } from '@/components/ui/skeleton'
import { ProjectsProvider } from './components/projects-provider'
import { ProjectsPrimaryButtons } from './components/projects-primary-buttons'
import { ProjectsTable } from './components/projects-table'
import { ProjectsDialogs } from './components/projects-dialogs'
import { useProjectList } from './hooks/use-projects'

const route = getRouteApi('/_authenticated/system/projects')

export function Projects() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const queryParams = {
    page: search.page || 1,
    page_size: search.page_size || 10,
    search: search.search || undefined,
    status: search.status || undefined,
  }

  const { data, isLoading, error } = useProjectList(queryParams)

  return (
    <ProjectsProvider>
      <Header fixed>
        <Search />
        <div className='ms-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 space-y-2'>
          <div className='flex items-center justify-between'>
            <h2 className='text-2xl font-bold tracking-tight'>Project Management</h2>
            <ProjectsPrimaryButtons />
          </div>
          <p className='text-muted-foreground'>Manage project information.</p>
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isLoading ? (
            <div className='space-y-4'>
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className='h-16 w-full' />
              ))}
            </div>
          ) : error ? (
            <div className='flex h-32 items-center justify-center text-muted-foreground'>
              Failed to load data. Please try again.
            </div>
          ) : (
            <ProjectsTable
              data={data?.list || []}
              search={search}
              navigate={(opts) => navigate(opts)}
              totalCount={data?.total || 0}
            />
          )}
        </div>
      </Main>

      <ProjectsDialogs />
    </ProjectsProvider>
  )
}
```

#### Supplementary Component: Table Column Definitions

Create `web/src/features/system/projects/components/projects-columns.tsx`:

```tsx
import { ColumnDef } from '@tanstack/react-table'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { DataTableRowActions } from '@/components/data-table/data-table-row-actions'
import { LongText } from '@/components/long-text'
import { Project } from '@/types/project'
import { useProjects } from './projects-provider'

export function useProjectColumns(): ColumnDef<Project>[] {
  const { setOpen, setCurrentRow } = useProjects()

  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label='Select row'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'name',
      header: 'Project Name',
      cell: ({ row }) => <LongText className='max-w-36'>{row.getValue('name')}</LongText>,
    },
    {
      accessorKey: 'code',
      header: 'Project Code',
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return <Badge variant={status === 'active' ? 'default' : 'secondary'}>{status}</Badge>
      },
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
    },
    {
      accessorKey: 'created_at',
      header: 'Created At',
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <DataTableRowActions
          row={row}
          onEdit={() => { setCurrentRow(row.original); setOpen('edit') }}
          onDelete={() => { setCurrentRow(row.original); setOpen('delete') }}
        />
      ),
    },
  ]
}
```

#### Supplementary Component: Data Table

Create `web/src/features/system/projects/components/projects-table.tsx`:

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { useReactTable, getCoreRowModel, getPaginationRowModel } from '@tanstack/react-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTable } from '@/components/data-table/data-table'
import { DataTablePagination } from '@/components/data-table/data-table-pagination'
import { Project } from '@/types/project'
import { useProjectColumns } from './projects-columns'

const route = getRouteApi('/_authenticated/system/projects')

interface ProjectsTableProps {
  data: Project[]
  search: Record<string, unknown>
  navigate: (opts: any) => void
  totalCount: number
}

export function ProjectsTable({ data, search, navigate, totalCount }: ProjectsTableProps) {
  const columns = useProjectColumns()

  const { pagination, onPaginationChange, columnFilters, onColumnFiltersChange } =
    useTableUrlState({ search, navigate })

  const table = useReactTable({
    data,
    columns,
    pageCount: Math.ceil(totalCount / (pagination.pageSize || 10)),
    state: { pagination, columnFilters },
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  return (
    <div className='space-y-4'>
      <DataTable table={table} />
      <DataTablePagination table={table} />
    </div>
  )
}
```

#### Supplementary Component: Dialog Aggregation

Create `web/src/features/system/projects/components/projects-dialogs.tsx`:

```tsx
import { useProjects } from './projects-provider'
import { ProjectFormDialog } from './project-form-dialog'

export function ProjectsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useProjects()

  return (
    <>
      <ProjectFormDialog
        key='project-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
      />
      {currentRow && (
        <ProjectFormDialog
          key={`project-edit-${currentRow.id}`}
          open={open === 'edit'}
          onOpenChange={() => {
            setOpen('edit')
            setTimeout(() => setCurrentRow(null), 500)
          }}
          currentRow={currentRow}
        />
      )}
    </>
  )
}
```

#### Supplementary Component: Header Action Buttons

Create `web/src/features/system/projects/components/projects-primary-buttons.tsx`:

```tsx
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermission } from '@/hooks/use-permission'
import { PERMISSIONS } from '@/constants/permissions'
import { useProjects } from './projects-provider'

export function ProjectsPrimaryButtons() {
  const { setOpen } = useProjects()
  const { hasPermission } = usePermission()

  return (
    <div className='flex gap-2'>
      {hasPermission(PERMISSIONS.SYSTEM.PROJECT.ADD) && (
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>Add Project</span> <Plus size={18} />
        </Button>
      )}
    </div>
  )
}
```

> **Key Point:** These four components follow a fixed pattern — refer to the corresponding files under `web/src/features/system/users/components/` and replace entity names with your own.

### Step 5: Implement Form Components

Create `web/src/features/system/projects/data/schema.ts`:

```typescript
import { z } from 'zod'

export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
  code: z.string(),
  description: z.string(),
  status: z.enum(['active', 'archived']),
  created_at: z.date(),
  updated_at: z.date(),
})

export type ProjectSchema = z.infer<typeof projectSchema>
```

Create `web/src/features/system/projects/components/project-form-dialog.tsx`:

```tsx
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import {
  Dialog, DialogClose, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { createProject, updateProject } from '@/services/projectApi'
import { useProjects } from './projects-provider'
import { toast } from 'sonner'

const formSchema = z.object({
  name: z.string().min(1, 'Please enter the project name'),
  code: z.string().min(1, 'Please enter the project code'),
  description: z.string(),
  status: z.enum(['active', 'archived']),
})

type ProjectForm = z.infer<typeof formSchema>

export function ProjectFormDialog({ mode }: { mode: 'create' | 'edit' }) {
  const {
    currentRow,
    showCreateDialog, setShowCreateDialog,
    showEditDialog, setShowEditDialog,
    setCurrentRow,
  } = useProjects()

  const isOpen = mode === 'create' ? showCreateDialog : showEditDialog
  const setOpen = mode === 'create' ? setShowCreateDialog : setShowEditDialog

  const queryClient = useQueryClient()

  const form = useForm<ProjectForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { name: '', code: '', description: '', status: 'active' },
  })

  // Populate form when editing
  useEffect(() => {
    if (mode === 'edit' && currentRow && showEditDialog) {
      form.reset({
        name: currentRow.name,
        code: currentRow.code,
        description: currentRow.description || '',
        status: currentRow.status as 'active' | 'archived',
      })
    }
  }, [currentRow, showEditDialog, mode, form])

  const mutation = useMutation({
    mutationFn: async (values: ProjectForm) => {
      if (mode === 'create') {
        return createProject(values)
      } else {
        return updateProject(currentRow!.id, values)
      }
    },
    onSuccess: () => {
      setOpen(false)
      form.reset()
      if (mode === 'edit') setCurrentRow(null)
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      toast.success(mode === 'create' ? 'Created successfully' : 'Updated successfully')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || (mode === 'create' ? 'Create failed' : 'Update failed'))
    },
  })

  const handleOpenChange = (open: boolean) => {
    setOpen(open)
    if (!open) {
      form.reset()
      if (mode === 'edit') setCurrentRow(null)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? 'Create Project' : 'Edit Project'}</DialogTitle>
          <DialogDescription>
            {mode === 'create' ? 'Create a new project' : 'Update project information'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit((v) => mutation.mutateAsync(v))} className='space-y-4'>
            <FormField control={form.control} name='name' render={({ field }) => (
              <FormItem>
                <FormLabel>Project Name *</FormLabel>
                <FormControl><Input placeholder='Enter project name' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='code' render={({ field }) => (
              <FormItem>
                <FormLabel>Project Code *</FormLabel>
                <FormControl><Input placeholder='Enter project code' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='status' render={({ field }) => (
              <FormItem>
                <FormLabel>Status</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                  <SelectContent>
                    <SelectItem value='active'>Active</SelectItem>
                    <SelectItem value='archived'>Archived</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='description' render={({ field }) => (
              <FormItem>
                <FormLabel>Description</FormLabel>
                <FormControl><Textarea placeholder='Enter project description' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <DialogFooter>
              <DialogClose asChild>
                <Button variant='outline' disabled={mutation.isPending}>Cancel</Button>
              </DialogClose>
              <Button type='submit' disabled={mutation.isPending}>
                {mutation.isPending ? 'Submitting...' : 'Confirm'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

**Key Points:**
- Uses React Hook Form + Zod for form validation
- `useEffect` populates the form with current row data in edit mode
- Mutation calls `invalidateQueries` on success to refresh the list + shows `toast` notification
- Create and edit share the same form component, differentiated by `mode`

### Step 6: Register the Route

Create `web/src/routes/_authenticated/system/projects.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Projects } from '@/features/system/projects'

const searchSchema = z.object({
  page: z.number().min(1).optional().default(1),
  page_size: z.number().min(1).max(100).optional().default(20),
  status: z.enum(['active', 'archived']).optional(),
  search: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/system/projects')({
  component: Projects,
  validateSearch: searchSchema,
})
```

**Key Points:**
- TanStack Router uses file-based routing — file path equals URL path
- `validateSearch` uses Zod to validate URL search params, providing automatic type safety
- The `/_authenticated/` prefix automatically applies the JWT route guard

After creating the route file, regenerate the route tree:

```bash
cd web && pnpm run dev
# TanStack Router will automatically regenerate routeTree.gen.ts
```

### Step 7: Add Menu and Permissions

#### Register Permission Constants

Add to `web/src/constants/permissions.ts`:

```typescript
export const PERMISSIONS = {
  SYSTEM: {
    // ... existing permissions
    PROJECT: {
      ADD: 'system:project:add',
      EDIT: 'system:project:edit',
      DELETE: 'system:project:delete',
    },
  },
} as const
```

#### Add Menu in the Admin Panel

Log in to the admin panel and go to **System Management → Menu Management**:

1. **Add Menu Item**: Name "Project Management", path `/system/projects`, parent "System Management"
2. **Assign Permission Buttons**: Add button permissions (add, edit, delete) to the menu item, with identifiers matching the `PERMISSIONS` constants
3. **Assign Roles**: In **Role Management**, assign the new menu to appropriate roles

#### Add API Resources in the Admin Panel

Go to **System Management → API Resource Management** and re-scan routes. The system will automatically discover the new API routes (format `GET:/api/v1/system/projects`, etc.), then assign them to the corresponding roles.

---

## Common Patterns and Tips

### File Upload

Backend uses `c.FormFile("file")` to receive files, calling the `pkg/storage` interface to save:

```go
file, err := c.FormFile("file")
if err != nil {
    c.JSON(http.StatusBadRequest, domain.RespError("File upload failed"))
    return
}
path, err := storage.Upload(file)
```

Frontend uses `FormData` for upload:

```typescript
const formData = new FormData()
formData.append('file', file)
await apiClient.post('/api/v1/upload', formData, {
  headers: { 'Content-Type': 'multipart/form-data' },
})
```

### Dictionary Selection

Frontend uses `getDictItemsByTypeCode` to fetch dictionary items as selection lists:

```typescript
import { getDictItemsByTypeCode } from '@/services/dictApi'

const statusOptions = await getDictItemsByTypeCode('project_status')
// Returns [{label: "In Progress", value: "active"}, ...]
```

### Tree Data

Refer to the menu module (`domain/menu.go`), which uses a `parent_id` field to build tree structures, with recursive rendering on the frontend.

### Batch Operations

Refer to the dictionary module's `useDeleteDictTypes` hook, using `Promise.all` for concurrent processing:

```typescript
export function useBatchDelete() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (ids: string[]) => {
      return Promise.all(ids.map(id => deleteProject(id)))
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      toast.success('Batch delete successful')
    },
  })
}
```

---

## Quality Assurance

### Backend

```bash
# Compile check
go build ./...

# Static analysis
go vet ./...

# Code formatting
go fmt ./...

# Unit tests
go test ./...

# Test coverage
go test ./... -cover
```

### Frontend

```bash
cd web

# TypeScript type checking
tsc -b

# ESLint check
pnpm run lint

# Format check
pnpm run format:check

# Auto-format
pnpm run format

# Unused dependency check
pnpm run knip
```

---

## Troubleshooting

### Compile Error: `ent/client.go` Errors

Run `go generate ./ent` to regenerate ORM code. Ensure field types in the Ent Schema match the Domain entity.

### Permissions Not Working

1. Confirm API resources have been scanned and assigned to roles in the admin panel
2. Confirm Casbin policies are updated (restart the service or call the refresh endpoint)
3. Check that routes have `casbinMiddleware.CheckAPIPermission()` applied

**Debugging Steps:**

```bash
# 1. Test with curl and observe the HTTP status code
# 401 = JWT Token invalid/expired; 403 = Casbin permission denied
curl -v -H "Authorization: Bearer YOUR_TOKEN" http://localhost:55667/api/v1/system/projects

# 2. Login to admin panel → API Resources, confirm new endpoints appear in the list
# If missing, restart the backend to trigger bootstrap.InitApiResources() auto-scan

# 3. Go to Role Management → Edit Role → Check the new API resources and menus
# Casbin policies are auto-updated on save

# 4. Re-login or refresh token to apply new permissions
```

> **Permission Model Note:** Shadmin uses a dual-layer permission model: the backend uses Casbin to control API access by `(userID, path, method)`; the frontend uses `PERMISSIONS` constant strings (e.g., `system:project:add`) to control button/menu visibility. They are linked through the "Role → Menu → API Resources" binding — when you assign menus to a role, you simultaneously assign the API resource permissions under those menus.

### Frontend Route 404

1. Confirm the route file is under `web/src/routes/_authenticated/`
2. Check that `routeTree.gen.ts` has been auto-updated (restart `pnpm dev`)
3. Confirm the `createFileRoute` path string matches the file location

### Swagger Docs Not Updated

Re-run `swag init -g main.go --output ./docs`, ensure the `@Router` annotation paths in Controller methods are correct.

### Frontend Request 401

Check if the JWT token has expired. Access tokens default to 3-hour expiry, configurable via `ACCESS_TOKEN_EXPIRY_HOUR` in `.env`.
