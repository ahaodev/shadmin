# Backend Development Patterns

Detailed Go/Gin/Ent code templates for Shadmin backend features. Read this when implementing backend layers.

## Domain Layer (`domain/<resource>.go`)

Define contracts first — everything else implements these.

```go
package domain

import (
    "context"
    "errors"
    "time"
)

// Entity — JSON tags use snake_case
type Project struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Code        string    `json:"code"`
    Description string    `json:"description"`
    Status      string    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// Create request — required fields use binding:"required"
type CreateProjectRequest struct {
    Name        string `json:"name" binding:"required"`
    Code        string `json:"code" binding:"required"`
    Description string `json:"description,omitempty"`
    Status      string `json:"status,omitempty"`
}

// Update request — pointer fields distinguish "not provided" from "set to empty"
type UpdateProjectRequest struct {
    Name        *string `json:"name,omitempty"`
    Code        *string `json:"code,omitempty"`
    Description *string `json:"description,omitempty"`
    Status      *string `json:"status,omitempty"`
}

// Query params — embed QueryParams for pagination support
type ProjectQueryParams struct {
    Name   string `json:"name,omitempty" form:"name"`
    Code   string `json:"code,omitempty" form:"code"`
    Status string `json:"status,omitempty" form:"status"`
    Search string `json:"search,omitempty" form:"search"`
    QueryParams
}

// Repository interface
type ProjectRepository interface {
    Create(ctx context.Context, project *Project) error
    GetByID(ctx context.Context, id string) (*Project, error)
    Fetch(ctx context.Context, params ProjectQueryParams) (*PagedResult[*Project], error)
    Update(ctx context.Context, id string, updates UpdateProjectRequest) error
    Delete(ctx context.Context, id string) error
}

// UseCase interface
type ProjectUseCase interface {
    CreateProject(ctx context.Context, req *CreateProjectRequest) (*Project, error)
    GetProjectByID(ctx context.Context, id string) (*Project, error)
    ListProjects(ctx context.Context, params ProjectQueryParams) (*PagedResult[*Project], error)
    UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) error
    DeleteProject(ctx context.Context, id string) error
}

// Sentinel errors — used by controller to map HTTP status codes
var (
    ErrProjectNotFound      = errors.New("project not found")
    ErrProjectAlreadyExists = errors.New("project already exists")
)

// Type alias for Swagger documentation
type ProjectPagedResult = PagedResult[*Project]
```

### QueryParams & PagedResult (built-in)

These are defined in `domain/request.go` and `domain/response.go`:

```go
// QueryParams — embedded in resource-specific query params
type QueryParams struct {
    IsAdmin  bool   `json:"-" form:"-"`
    Page     int    `json:"page" form:"page"`
    PageSize int    `json:"page_size" form:"page_size"`
    SortBy   string `json:"sort_by" form:"sort_by"`
    Order    string `json:"order" form:"order"`
}

// ValidateQueryParams — sets defaults: Page=1, PageSize=10, Max=10000
func ValidateQueryParams(params *QueryParams) error

// PagedResult — generic paginated response
type PagedResult[T any] struct {
    List       []T `json:"list"`
    Total      int `json:"total"`
    Page       int `json:"page"`
    PageSize   int `json:"page_size"`
    TotalPages int `json:"total_pages"`
}

func NewPagedResult[T any](data []T, total, page, pageSize int) *PagedResult[T]

// Response helpers
func RespSuccess(data interface{}) Response  // Code=0, Msg="OK"
func RespError(msg interface{}) Response     // Code=1, accepts string or error
```

## Ent Schema (`ent/schema/<resource>.go`)

```go
package schema

import (
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
            DefaultFunc(func() string { return xid.New().String() }),
        field.String("name").
            NotEmpty().
            Comment("Project name"),
        field.String("code").
            Unique().
            NotEmpty().
            Comment("Project code"),
        field.String("description").
            Optional().
            Default(""),
        field.String("status").
            Default("active").
            Comment("active or archived"),
        field.Time("created_at").
            Default(time.Now),
        field.Time("updated_at").
            Default(time.Now).
            UpdateDefault(time.Now),
    }
}

func (Project) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("name"),
        index.Fields("status"),
    }
}
```

After creating or modifying, always run:
```bash
go generate ./ent
```

## Repository (`repository/<resource>_repository.go`)

```go
package repository

import (
    "context"
    "fmt"

    "shadmin/domain"
    "shadmin/ent"
    "shadmin/ent/project"
)

type entProjectRepository struct {
    client *ent.Client
}

func NewProjectRepository(client *ent.Client) domain.ProjectRepository {
    return &entProjectRepository{client: client}
}

// Convert Ent entity to domain entity
func (r *entProjectRepository) convertToDomain(e *ent.Project) *domain.Project {
    return &domain.Project{
        ID:          e.ID,
        Name:        e.Name,
        Code:        e.Code,
        Description: e.Description,
        Status:      e.Status,
        CreatedAt:   e.CreatedAt,
        UpdatedAt:   e.UpdatedAt,
    }
}

func (r *entProjectRepository) Create(ctx context.Context, p *domain.Project) error {
    created, err := r.client.Project.Create().
        SetName(p.Name).
        SetCode(p.Code).
        SetDescription(p.Description).
        SetStatus(p.Status).
        Save(ctx)
    if err != nil {
        return fmt.Errorf("failed to create project: %w", err)
    }
    p.ID = created.ID
    p.CreatedAt = created.CreatedAt
    p.UpdatedAt = created.UpdatedAt
    return nil
}

func (r *entProjectRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
    e, err := r.client.Project.Get(ctx, id)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, domain.ErrProjectNotFound
        }
        return nil, fmt.Errorf("failed to get project: %w", err)
    }
    return r.convertToDomain(e), nil
}

func (r *entProjectRepository) Fetch(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[*domain.Project], error) {
    domain.ValidateQueryParams(&params.QueryParams)

    query := r.client.Project.Query()

    // Apply filters
    if params.Name != "" {
        query = query.Where(project.NameContains(params.Name))
    }
    if params.Status != "" {
        query = query.Where(project.StatusEQ(params.Status))
    }
    if params.Search != "" {
        query = query.Where(
            project.Or(
                project.NameContains(params.Search),
                project.CodeContains(params.Search),
            ),
        )
    }

    // Clone query for count (before pagination)
    total, err := query.Clone().Count(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to count projects: %w", err)
    }

    // Apply sorting
    if params.SortBy != "" {
        if params.Order == "desc" {
            query = query.Order(ent.Desc(params.SortBy))
        } else {
            query = query.Order(ent.Asc(params.SortBy))
        }
    } else {
        query = query.Order(ent.Desc(project.FieldCreatedAt))
    }

    // Apply pagination
    offset := (params.GetPage() - 1) * params.GetPageSize()
    entities, err := query.
        Offset(offset).
        Limit(params.GetPageSize()).
        All(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch projects: %w", err)
    }

    // Convert to domain entities
    items := make([]*domain.Project, len(entities))
    for i, e := range entities {
        items[i] = r.convertToDomain(e)
    }

    return domain.NewPagedResult(items, total, params.GetPage(), params.GetPageSize()), nil
}

func (r *entProjectRepository) Update(ctx context.Context, id string, updates domain.UpdateProjectRequest) error {
    mutation := r.client.Project.UpdateOneID(id)

    // Check pointer fields for partial updates
    if updates.Name != nil {
        mutation = mutation.SetName(*updates.Name)
    }
    if updates.Code != nil {
        mutation = mutation.SetCode(*updates.Code)
    }
    if updates.Description != nil {
        mutation = mutation.SetDescription(*updates.Description)
    }
    if updates.Status != nil {
        mutation = mutation.SetStatus(*updates.Status)
    }

    _, err := mutation.Save(ctx)
    if err != nil {
        if ent.IsNotFound(err) {
            return domain.ErrProjectNotFound
        }
        return fmt.Errorf("failed to update project: %w", err)
    }
    return nil
}

func (r *entProjectRepository) Delete(ctx context.Context, id string) error {
    err := r.client.Project.DeleteOneID(id).Exec(ctx)
    if err != nil {
        if ent.IsNotFound(err) {
            return domain.ErrProjectNotFound
        }
        return fmt.Errorf("failed to delete project: %w", err)
    }
    return nil
}
```

### Key repository patterns:
- `ent.IsNotFound(err)` → return domain sentinel error
- Clone query for count before applying offset/limit
- Pointer checks for partial updates
- Default sort by `created_at` descending
- Error wrapping with `%w`

## Usecase (`usecase/<resource>_usecase.go`)

```go
package usecase

import (
    "context"
    "fmt"
    "time"

    "shadmin/domain"
    "shadmin/ent"
)

type projectUsecase struct {
    client            *ent.Client
    projectRepository domain.ProjectRepository
    contextTimeout    time.Duration
}

func NewProjectUsecase(client *ent.Client, repo domain.ProjectRepository, timeout time.Duration) domain.ProjectUseCase {
    return &projectUsecase{client: client, projectRepository: repo, contextTimeout: timeout}
}

func (pu *projectUsecase) CreateProject(ctx context.Context, req *domain.CreateProjectRequest) (*domain.Project, error) {
    ctx, cancel := context.WithTimeout(ctx, pu.contextTimeout)
    defer cancel()

    // Set defaults
    if req.Status == "" {
        req.Status = "active"
    }

    // Business validation here (uniqueness, enum checks, etc.)

    p := &domain.Project{
        Name:        req.Name,
        Code:        req.Code,
        Description: req.Description,
        Status:      req.Status,
    }

    if err := pu.projectRepository.Create(ctx, p); err != nil {
        return nil, fmt.Errorf("failed to create project: %w", err)
    }
    return p, nil
}

func (pu *projectUsecase) GetProjectByID(ctx context.Context, id string) (*domain.Project, error) {
    ctx, cancel := context.WithTimeout(ctx, pu.contextTimeout)
    defer cancel()
    return pu.projectRepository.GetByID(ctx, id)
}

func (pu *projectUsecase) ListProjects(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[*domain.Project], error) {
    ctx, cancel := context.WithTimeout(ctx, pu.contextTimeout)
    defer cancel()

    domain.ValidateQueryParams(&params.QueryParams)
    return pu.projectRepository.Fetch(ctx, params)
}

func (pu *projectUsecase) UpdateProject(ctx context.Context, id string, req domain.UpdateProjectRequest) error {
    ctx, cancel := context.WithTimeout(ctx, pu.contextTimeout)
    defer cancel()
    return pu.projectRepository.Update(ctx, id, req)
}

func (pu *projectUsecase) DeleteProject(ctx context.Context, id string) error {
    ctx, cancel := context.WithTimeout(ctx, pu.contextTimeout)
    defer cancel()
    return pu.projectRepository.Delete(ctx, id)
}
```

### Key usecase patterns:
- Every method: `context.WithTimeout` + `defer cancel()`
- Business validation belongs here (status enums, uniqueness, cross-entity)
- Error wrapping with `%w`
- Constructor takes `*ent.Client`, repository interface, timeout duration

## Controller (`api/controller/<resource>_controller.go`)

```go
package controller

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "shadmin/domain"
)

type ProjectController struct {
    ProjectUseCase domain.ProjectUseCase
}

// @Summary List projects
// @Tags Project
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param search query string false "Search keyword"
// @Param status query string false "Filter by status"
// @Success 200 {object} domain.Response{data=domain.ProjectPagedResult}
// @Failure 500 {object} domain.Response
// @Router /api/v1/system/project [get]
func (pc *ProjectController) ListProjects(c *gin.Context) {
    var params domain.ProjectQueryParams
    if err := c.ShouldBindQuery(&params); err != nil {
        c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
        return
    }

    result, err := pc.ProjectUseCase.ListProjects(c, params)
    if err != nil {
        c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
        return
    }
    c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// @Summary Create project
// @Tags Project
// @Security BearerAuth
// @Param body body domain.CreateProjectRequest true "Project data"
// @Success 201 {object} domain.Response{data=domain.Project}
// @Failure 400 {object} domain.Response
// @Router /api/v1/system/project [post]
func (pc *ProjectController) CreateProject(c *gin.Context) {
    var req domain.CreateProjectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
        return
    }

    project, err := pc.ProjectUseCase.CreateProject(c, &req)
    if err != nil {
        if errors.Is(err, domain.ErrProjectAlreadyExists) {
            c.JSON(http.StatusConflict, domain.RespError(err.Error()))
            return
        }
        c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
        return
    }
    c.JSON(http.StatusCreated, domain.RespSuccess(project))
}

// @Summary Get project by ID
// @Tags Project
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} domain.Response{data=domain.Project}
// @Failure 404 {object} domain.Response
// @Router /api/v1/system/project/{id} [get]
func (pc *ProjectController) GetProject(c *gin.Context) {
    id := c.Param("id")

    project, err := pc.ProjectUseCase.GetProjectByID(c, id)
    if err != nil {
        if errors.Is(err, domain.ErrProjectNotFound) {
            c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
            return
        }
        c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
        return
    }
    c.JSON(http.StatusOK, domain.RespSuccess(project))
}

// @Summary Update project
// @Tags Project
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body domain.UpdateProjectRequest true "Update data"
// @Success 200 {object} domain.Response
// @Failure 404 {object} domain.Response
// @Router /api/v1/system/project/{id} [put]
func (pc *ProjectController) UpdateProject(c *gin.Context) {
    id := c.Param("id")

    var req domain.UpdateProjectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
        return
    }

    if err := pc.ProjectUseCase.UpdateProject(c, id, req); err != nil {
        if errors.Is(err, domain.ErrProjectNotFound) {
            c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
            return
        }
        c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
        return
    }
    c.JSON(http.StatusOK, domain.RespSuccess(nil))
}

// @Summary Delete project
// @Tags Project
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} domain.Response
// @Failure 404 {object} domain.Response
// @Router /api/v1/system/project/{id} [delete]
func (pc *ProjectController) DeleteProject(c *gin.Context) {
    id := c.Param("id")

    if err := pc.ProjectUseCase.DeleteProject(c, id); err != nil {
        if errors.Is(err, domain.ErrProjectNotFound) {
            c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
            return
        }
        c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
        return
    }
    c.JSON(http.StatusOK, domain.RespSuccess(nil))
}
```

### HTTP status code mapping:
| Scenario | Status | Response |
|----------|--------|----------|
| Success | 200 OK | `domain.RespSuccess(data)` |
| Created | 201 Created | `domain.RespSuccess(entity)` |
| Bad request/validation | 400 Bad Request | `domain.RespError(err.Error())` |
| Not found | 404 Not Found | `domain.RespError(err.Error())` |
| Already exists | 409 Conflict | `domain.RespError(err.Error())` |
| Server error | 500 Internal Server Error | `domain.RespError(err.Error())` |

### Swagger annotation pattern:
- `@Summary` — brief description
- `@Tags` — resource name (used for grouping)
- `@Security BearerAuth` — required for protected endpoints
- `@Param` — query/path/body parameters
- `@Success`/`@Failure` — response with type hint
- `@Router` — path and method

## Routes (`api/route/`)

Add a setup function in the appropriate route file (usually `system_routes.go`):

```go
func (pr *ProtectedRoutes) setupProjectManagement(systemGroup *gin.RouterGroup, casbinMiddleware *middleware.CasbinMiddleware) {
    group := systemGroup.Group("/project")
    group.Use(casbinMiddleware.CheckAPIPermission())
    ctrl := pr.factory.CreateProjectController()

    group.GET("", ctrl.ListProjects)
    group.POST("", ctrl.CreateProject)
    group.GET("/:id", ctrl.GetProject)
    group.PUT("/:id", ctrl.UpdateProject)
    group.DELETE("/:id", ctrl.DeleteProject)
}
```

Then call it from `setupSystemRoutes`:

```go
func (pr *ProtectedRoutes) setupSystemRoutes(group *gin.RouterGroup, casbinMiddleware *middleware.CasbinMiddleware) {
    systemGroup := group.Group("/system")
    // ... existing setup calls ...
    pr.setupProjectManagement(systemGroup, casbinMiddleware)
}
```

### REST conventions:
| Method | Path | Action |
|--------|------|--------|
| GET | `/system/project` | List (paginated) |
| POST | `/system/project` | Create |
| GET | `/system/project/:id` | Get by ID |
| PUT | `/system/project/:id` | Update |
| DELETE | `/system/project/:id` | Delete |

## Factory (`api/route/factory.go`)

```go
func (f *ControllerFactory) CreateProjectController() *controller.ProjectController {
    repo := repository.NewProjectRepository(f.db)
    uc := usecase.NewProjectUsecase(f.db, repo, f.timeout)
    return &controller.ProjectController{ProjectUseCase: uc}
}
```

### ControllerFactory fields available:
- `f.db` — `*ent.Client` for database access
- `f.app` — `*bootstrap.Application` (includes `CasManager`, `FileRepo`, `ApiEngine`)
- `f.timeout` — `time.Duration` for context timeouts

If a controller needs Casbin: pass `f.app.CasManager` to the repository.

## Auth & Middleware

### JWT middleware context keys:
- `x-user-id` — user's unique ID
- `x-user-name` — username
- `x-user-email` — email
- `x-user-is-admin` — boolean admin flag
- `x-user-roles` — comma-separated role names

### Casbin middleware:
`CheckAPIPermission()` checks `(userID, requestPath, requestMethod)` against Casbin policies.
- Returns **403 Forbidden** if permission denied
- Returns **500** if check fails
- Auto-skips whitelisted paths (health, swagger, etc.)

### API resource auto-scan:
On startup, `bootstrap.InitApiResources` scans all Gin routes and creates DB records:
- ID format: `METHOD:/api/v1/path` (e.g., `GET:/api/v1/system/project`)
- Skips public, swagger, and whitelisted paths
- Preserves existing menu→API resource associations
