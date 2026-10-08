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

`domain.QueryParams` embeds into resource query params; `qp.Paginate()` sets defaults (Page=1, PageSize=10, Max=10000) and returns `(offset, limit)`.

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
- **Fetch**: call `params.Paginate()` first to get offset/limit; clone query for count before applying them; default sort by `created_at` DESC
- **Update**: check each pointer field before calling `Set*()`; wrap `ent.IsNotFound` → sentinel error
- All errors wrapped with `fmt.Errorf("...: %w", err)`
- Methods that can run inside a `domain.UnitOfWork` must query through `clientFromContext(ctx, r.client)` (not `r.client`), so they join the transaction carried in `ctx`

## Usecase (`usecase/<resource>_usecase.go`)

- Constructor: `func New<Resource>Usecase(repo domain.ResourceRepository, timeout time.Duration) domain.ResourceUseCase`
- Dependencies are domain interfaces only — never `*ent.Client`, `gin`, or concrete `internal/` types (see Ports below)
- Every method: `ctx, cancel := context.WithTimeout(ctx, uc.contextTimeout); defer cancel()`
- Business validation lives here (status enums, uniqueness checks, cross-entity rules)
- Set defaults on create requests (e.g., `if req.Status == "" { req.Status = "active" }`)
- Errors wrapped with `fmt.Errorf("...: %w", err)`

### Ports and transactions

Usecases depend only on interfaces declared in `domain/`. Implementations live in `repository/` or `internal/` and are injected in `bootstrap/usecases.go`.

| Port | Implemented by |
|------|----------------|
| `TokenIssuer`, `TokenBlacklist` | `internal/tokenservice`, `internal/auth` |
| `LoginSecurity`, `UserIdentityCodeStore` | `internal/auth` |
| `SlideCaptchaManager` | `internal/captcha` |
| `AuthorizationSource` | `repository` (ent), consumed by `internal/casbin` |
| `UnitOfWork` | `repository` (ent) |

When several repositories must change atomically, the usecase wraps the work in `UnitOfWork.Do`. The transaction travels in `ctx`, so every repository call inside the callback must use the callback's `txCtx`:

```go
err := u.uow.Do(ctx, func(txCtx context.Context) error {
    return u.userRepository.CreateWithRoles(txCtx, user, []string{roleID})
})
```

`repository.WithAuthorizationTx` joins a transaction already in `ctx` and advances the authorization generation once.

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

Add a `setup<Resource>Management(systemGroup *gin.RouterGroup, casbinMiddleware)` method in `system_routes.go`, then call it from `setupSystemRoutes` (in `protected.go`).

REST convention:
- `GET    /system/resource`     — list (paginated)
- `POST   /system/resource`     — create
- `GET    /system/resource/:id` — get by ID
- `PUT    /system/resource/:id` — update
- `DELETE /system/resource/:id` — delete

All system routes use `group.Use(casbinMiddleware.CheckAPIPermission())`.

## Wiring (`bootstrap/usecases.go` → `api/route/factory.go`)

Wiring has two steps, each with one job.

1. **Usecase wiring** — `bootstrap/usecases.go` is the only place where repositories are turned into usecases. Add a field to `bootstrap.Usecases` and construct it in `newUsecases`:
```go
Resource: usecase.NewResourceUsecase(repository.NewResourceRepository(db), timeout),
```
2. **Controller factory** — `api/route/factory.go` builds controllers from usecases only:
```go
func (f *ControllerFactory) CreateResourceController() *controller.ResourceController {
    return &controller.ResourceController{ResourceUseCase: f.uc.Resource}
}
```

The factory must not import `ent` or `repository`. Its fields are `f.uc` (`*bootstrap.Usecases`) and `f.app` (`*bootstrap.Application`, for configuration values such as `f.app.Env.IdentityRedirectURL`).

## Auth & Middleware

### JWT context keys (injected by `JwtAuthMiddleware`)
- `x-user-id`, `x-user-name`, `x-user-email`, `x-user-is-admin`, `x-user-roles`

### Casbin middleware
`CheckAPIPermission()` enforces `(userID, requestPath, requestMethod)` — returns 403 if denied. Whitelisted paths (health, swagger) are auto-skipped.

### API resource and Casbin snapshot sync
`bootstrap.InitApiResources` scans Gin routes and persists records with deterministic IDs (`METHOD:/api/v1/path`), preserving menu→API-resource associations. A changed route inventory advances `authz_state.generation` in the same transaction.

Ent user/role/menu/API-resource relations are the authorization source of truth. Permission-affecting writes must go through repository methods that update those relations and advance the generation in the same DB transaction (`repository.WithAuthorizationTx` or the equivalent transaction-scoped helper). Each process polls the shared generation and builds a complete in-memory Casbin snapshot; publish the new Enforcer only after the build succeeds. Do not use adapter AutoSave or mutate a published Enforcer. A stale snapshot is fail-closed by the Casbin middleware (HTTP 503) until a fresh snapshot is available.
