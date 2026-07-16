# Backend Development Patterns

Go/Gin/Ent patterns for Shadmin backend. Read this when implementing backend layers.

## Domain Layer (`domain/<resource>.go`)

Define contracts first — everything else implements these.

**Entity struct** — JSON tags use `snake_case`, timestamps are `time.Time`.

**Create request** — required fields use `binding:"required"`.

**Update request** — all fields are pointers (`*string`, `*int`) to distinguish "not provided" from "set to empty".

**Query params** — embed `domain.QueryParams` for built-in pagination support.

**Repository interface** — `Create`, `GetByID`, `Fetch(params QueryParams) (*PagedResult[*T], error)`, `Update`, `Delete`.

**UseCase interface** — mirrors repository but accepts request DTOs and applies business logic.

**Sentinel errors** — `ErrResourceNotFound`, `ErrResourceAlreadyExists` — used by controller to map HTTP status.

**Swagger type alias** — `type ResourcePagedResult = PagedResult[*Resource]` for Swagger annotations.

### Built-in domain helpers

`domain.QueryParams` embeds into resource query params; `domain.ValidateQueryParams()` sets defaults (Page=1, PageSize=10, Max=10000).

`domain.NewPagedResult(items, total, page, pageSize)` builds paginated response.

`domain.RespSuccess(data)` → `{code:0, msg:"OK", data:...}` / `domain.RespError(msg)` → `{code:1, msg:...}`.

## Ent Schema (`ent/schema/<resource>.go`)

- ID: `field.String("id").DefaultFunc(func() string { return xid.New().String() })`
- Timestamps: `field.Time("created_at").Default(time.Now)` / `.UpdateDefault(time.Now)` for updated_at
- Unique fields: `.Unique()` — Ent enforces at DB level
- Optional fields: `.Optional().Default("")`
- Add relevant indexes for frequently filtered fields

Run `go generate ./ent` after any schema change.

## Repository (`repository/<resource>_repository.go`)

- Struct holds `*ent.Client`; constructor returns `domain.ResourceRepository`
- `convertToDomain()` private method converts Ent entity to domain entity
- **GetByID**: wrap `ent.IsNotFound(err)` → return domain sentinel error
- **Fetch**: call `domain.ValidateQueryParams` first; clone query for count before applying offset/limit; default sort by `created_at` DESC
- **Update**: check each pointer field before calling `Set*()`; wrap `ent.IsNotFound` → sentinel error
- All errors wrapped with `fmt.Errorf("...: %w", err)`

## Usecase (`usecase/<resource>_usecase.go`)

- Constructor: `func New<Resource>Usecase(client *ent.Client, repo domain.ResourceRepository, timeout time.Duration) domain.ResourceUseCase`
- Every method: `ctx, cancel := context.WithTimeout(ctx, uc.contextTimeout); defer cancel()`
- Business validation lives here (status enums, uniqueness checks, cross-entity rules)
- Set defaults on create requests (e.g., `if req.Status == "" { req.Status = "active" }`)
- Errors wrapped with `fmt.Errorf("...: %w", err)`

## Controller (`api/controller/<resource>_controller.go`)

- Struct holds `domain.ResourceUseCase`; no business logic
- Parse: `c.ShouldBindQuery` for GET params, `c.ShouldBindJSON` for request body, `c.Param("id")` for path params
- Return `domain.RespSuccess(data)` / `domain.RespError(err.Error())` with correct HTTP status
- Use `errors.Is(err, domain.ErrResourceNotFound)` to map sentinel errors to status codes
- Every handler has Swagger annotations (`@Summary`, `@Tags`, `@Security BearerAuth`, `@Param`, `@Success`, `@Failure`, `@Router`)

### HTTP status mapping

| Scenario | Status |
|----------|--------|
| Success (read/update/delete) | 200 |
| Success (create) | 201 |
| Validation error | 400 |
| Not found | 404 |
| Already exists / conflict | 409 |
| Server error | 500 |

## Routes (`api/route/`)

Add a `setup<Resource>Management(systemGroup *gin.RouterGroup, casbinMiddleware)` method in `protected.go` (or system routes file), then call it from `setupSystemRoutes`.

REST convention:
- `GET    /system/resource`     — list (paginated)
- `POST   /system/resource`     — create
- `GET    /system/resource/:id` — get by ID
- `PUT    /system/resource/:id` — update
- `DELETE /system/resource/:id` — delete

All system routes use `group.Use(casbinMiddleware.CheckAPIPermission())`.

## Factory (`api/route/factory.go`)

Wire in one method:
```go
func (f *ControllerFactory) Create<Resource>Controller() *controller.ResourceController {
    repo := repository.NewResourceRepository(f.db)
    uc := usecase.NewResourceUsecase(f.db, repo, f.timeout)
    return &controller.ResourceController{ResourceUseCase: uc}
}
```

Available factory fields: `f.db` (`*ent.Client`), `f.app` (`*bootstrap.Application` — has `CasManager`, `FileRepo`, `ApiEngine`), `f.timeout` (`time.Duration`).

## Auth & Middleware

### JWT context keys (injected by `JwtAuthMiddleware`)
- `x-user-id`, `x-user-name`, `x-user-email`, `x-user-is-admin`, `x-user-roles`

### Casbin middleware
`CheckAPIPermission()` enforces `(userID, requestPath, requestMethod)` — returns 403 if denied. Whitelisted paths (health, swagger) are auto-skipped.

### API resource auto-scan
On startup, `bootstrap.InitApiResources` scans all Gin routes and persists records with ID format `METHOD:/api/v1/path`. Existing menu→API resource associations are preserved across rebuilds.
