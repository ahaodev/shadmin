# Backend Patterns

Go/Gin/Ent. Reference implementation: Department (`domain/department.go`, `repository/department_repository.go`, `usecase/department_usecase.go`, `api/controller/department_controller.go`).

## Domain (`domain/<resource>.go`)

Contracts first; everything else implements them.
- Entity: `snake_case` JSON tags, `time.Time` timestamps.
- `Create*Request`: `binding:"required,max=..."`. `Update*Request`: pointer fields (`*string`) so "absent" ≠ "empty".
- Filter: struct with `form:"..."` tags embedding `domain.QueryParams` (`Page`, `PageSize`, `SortBy`, `Order`). `Paginate()` defaults page=1, size=10, max 10000 and returns `(offset, limit)`.
- Repository interface: CRUD + list. List returns `*PagedResult[*T]` (see `UserRepository.Query`); tree/flat listings may return slices (`FetchTree`, `FetchList`).
- UseCase interface: same shape, takes request DTOs. Naming in the repo is mixed (`UseCase`/`Usecase`) — follow the neighbouring file.
- Sentinel errors: `ErrXNotFound`, `ErrXNameExists`, ... Swagger alias: `type XPagedResult = PagedResult[*X]`.
- Helpers (`domain/response.go`): `RespSuccess(data)`, `RespError(msg|err)`, `NewPagedResult(list, total, page, pageSize)` (empty list serializes as `[]`).

## Ent schema (`ent/schema/<resource>.go`)
- ID: `field.String("id").DefaultFunc(func() string { return xid.New().String() }).Unique().Immutable()`.
- `created_at`: `Default(time.Now).Immutable()`; `updated_at`: `Default(time.Now).UpdateDefault(time.Now)`.
- Status: `field.Enum("status").Values("active","inactive").Default("active")`. Optional text: `.Default("")`.
- Declare edges, unique/filter indexes (`index.Fields("name","parent_id").Unique()`).
- Run `go generate ./ent`; generated code is committed.

## Repository
- Struct holds `*ent.Client`; `NewXRepository(client)` returns the domain interface; conversion via package funcs like `entDepartmentToDomain`.
- Not found: `ent.IsNotFound(err)` → domain sentinel. Unique violation: `ent.IsConstraintError(err)` → `ErrXExists`.
- List: `filter.Paginate()` for offset/limit; count on a cloned query before applying them; sort with `ApplySorting(sortBy, order, fieldMap, defaultField)` (`repository/helpers.go`); `buildTree`/`mapSlice` for trees/conversion.
- Update: set only non-nil pointer fields.
- Wrap other errors `fmt.Errorf("...: %w", err)`.
- Methods that may run inside a `UnitOfWork` must query via `clientFromContext(ctx, r.client)`, not `r.client`.

## Usecase
- `NewXUsecase(repo domain.XRepository, timeout time.Duration) domain.XUseCase`; unexported struct.
- Depends only on `domain` interfaces. Implementations are injected in `bootstrap/usecases.go`:

| Port | Implementation |
|---|---|
| `TokenIssuer` | `internal/tokenservice` |
| `TokenBlacklist`, `LoginSecurity`, `UserIdentityCodeStore` | `internal/auth` |
| `SlideCaptchaManager` | `internal/captcha` |
| `InvitationMailer` | `internal/mailer` |
| `AuthorizationSource`, `UnitOfWork` | `repository` |

- Each method starts with `ctx, cancel := context.WithTimeout(ctx, u.contextTimeout); defer cancel()`.
- Validation, defaults (`constants.StatusActive`), uniqueness and cross-entity rules live here.
- Atomic multi-repo work: `uow.Do(ctx, func(txCtx) error {...})`; every repository call inside must use `txCtx`. `repository.WithAuthorizationTx` joins a transaction already in `ctx`.

## Controller
- Struct holds the usecase (field names vary: `DepartmentUseCase`, `ResourceUsecase`, ...).
- Parse: `ShouldBindQuery` (filters), `controller.MustBindJSON(c, &req)` (writes 400 on failure), `c.Param("id")`. `controller.BindQueryParams(c)` extracts bare pagination.
- Identity from context: `c.GetString(constants.UserID)`, `c.GetBool(constants.IsAdmin)` (`internal/constants/custom_header.go`; set by `JwtAuthMiddleware`: `x-user-id`, `x-user-name`, `x-user-email`, `x-is-admin`, `x-user-roles`, `x-user-subject`).
- Map errors with `errors.Is`. Statuses: 200 ok · 201 create · 400 validation/bad state · 403 forbidden · 404 not found · 409 conflict · 500 other.
- Swagger on every handler: `@Summary @Description @Tags @Accept @Produce @Security BearerAuth @Param @Success @Failure @Router` (router path relative to `/api/v1`).

## Routes (`api/route/`)
Add `setup<Resource>Management(systemGroup, casbinMiddleware)` in `system_routes.go`, call it from `setupSystemRoutes` in `protected.go`:
```go
g := systemGroup.Group("/resource")
g.Use(casbinMiddleware.CheckAPIPermission())
c := pr.factory.CreateResourceController()
g.GET("", c.List); g.POST("", c.Create)
g.GET("/:id", c.Get); g.PUT("/:id", c.Update); g.DELETE("/:id", c.Delete)
```
`public.go` holds unauthenticated routes. Non-system protected routes (`/profile`, `/resources`) skip Casbin via `constants.BasicProtectedAPIPaths`.

## Wiring
1. `bootstrap/usecases.go` — the only place repositories become usecases: add a field to `Usecases` and construct it in `newUsecases`.
2. `api/route/factory.go` — controllers from usecases only:
```go
func (f *ControllerFactory) CreateResourceController() *controller.ResourceController {
    return &controller.ResourceController{ResourceUseCase: f.uc.Resource}
}
```
Config values come from `f.app.Env`.

Casbin middleware returns 403 on deny and 503 when the snapshot is stale. Authorization-affecting writes: see SKILL.md §5.
