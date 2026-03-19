# Copilot Instructions — Shadmin

Shadmin is a full-stack RBAC admin dashboard: Go backend (Gin + Ent ORM + Casbin) and React 19 frontend (Shadcn UI + TanStack Router/Query + Zustand).

## Build, Test, and Lint

### Backend (Go)

```bash
go run .                        # Dev server on :55667
go build -o shadmin .           # Production build
go test ./...                   # All tests
go test ./usecase/...           # Single package
go test -run TestFuncName ./pkg/...  # Single test
go fmt ./... && go vet ./...    # Format + lint (required before commit)
go generate ./ent               # Regenerate Ent after schema changes
go generate ./...               # Ent + Swagger (needs swag CLI)
```

### Frontend (web/)

```bash
cd web
pnpm install                    # Install deps (npm also works)
pnpm dev                        # Dev server on :5173, proxies /api to :55667
pnpm build                      # Type-check + Vite build → web/dist/
pnpm lint                       # ESLint
pnpm format:check               # Prettier check
pnpm format                     # Prettier auto-fix
```

### Pre-commit hook

`.githooks/pre-commit` runs `go fmt`, `go vet`, `go test`, and frontend lint/format automatically.

## Architecture

### Backend — Clean Architecture Layers

```
main.go → cmd.Run() → bootstrap.App() → api.SetupRoutes() → api.Run()
```

Request flow: **Route → Middleware (JWT + Casbin) → Controller → Usecase → Repository → Ent/DB**

| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| Domain | `domain/` | Entities, DTOs, Repository/UseCase interfaces, errors, `RespSuccess()`/`RespError()` response helpers |
| Schema | `ent/schema/` | Ent ORM schema definitions (run `go generate ./ent` after changes) |
| Repository | `repository/` | Ent data access + domain↔ent model conversion |
| Usecase | `usecase/` | Business logic with `context.WithTimeout`, error wrapping (`%w`) |
| Controller | `api/controller/` | HTTP parsing only (`ShouldBindJSON`/`Query`/`Param`), Swagger annotations, no business logic |
| Route | `api/route/` | Route registration, middleware wiring |
| Factory | `api/route/factory.go` | DI: creates Repository → Usecase → Controller chains |
| Bootstrap | `bootstarp/` | App init, DB, Casbin, storage, seed data (note: directory has a typo) |

### Frontend — Feature-Based Structure

```
web/src/
├── routes/            # TanStack file-based routing (auto code-split)
│   ├── (auth)/        # Public auth routes
│   ├── (errors)/      # Error pages
│   └── _authenticated/ # Protected routes (JWT guard in beforeLoad)
├── features/          # Feature modules (pages, components, hooks, schemas)
├── services/          # API wrappers using Axios (return response.data.data)
├── stores/            # Zustand stores (auth-store with permissions)
├── components/ui/     # Shadcn UI primitives
├── hooks/             # Custom hooks (useDebounce, usePermission, useTableUrlState)
├── types/             # Shared TypeScript interfaces
├── lib/               # Utilities (cn(), handleServerError, menu-utils)
└── context/           # Providers (theme, font, layout, search)
```

### Auth & Permissions

- **Authentication**: JWT access + refresh tokens. Middleware extracts claims into Gin context (`x-user-*` keys).
- **Authorization**: Casbin checks `(userID, path, method)` on `/api/v1/system/*` routes via `CheckAPIPermission()` middleware.
- **Frontend**: `auth-store` exposes `hasPermission()`, `hasRole()`, `canAccessMenu()`. Use `PermissionButton`/`PermissionGuard` for gated UI.
- **Login security**: 3 failed attempts → 1-minute lockout (`internal/login_security.go`).

### Database & Storage

- Default SQLite (`.database/data.db`). Set `DB_TYPE=postgres|mysql` + `DB_DSN` for others.
- Auto-migration on startup.
- File storage: `STORAGE_TYPE=disk|minio` with abstract interface in `domain/`.

## Key Conventions

### Adding a Backend Feature (layer-by-layer order)

1. `domain/<resource>.go` — Entity struct, Create/Update request DTOs, QueryFilter (embed `domain.QueryParams`), Repository + UseCase interfaces, sentinel errors
2. `ent/schema/<resource>.go` — DB schema → run `go generate ./ent`
3. `repository/<resource>_repository.go` — Ent CRUD, domain↔ent converters, pagination via `domain.ValidateQueryParams()`
4. `usecase/<resource>_usecase.go` — `context.WithTimeout`, validation, cross-repo orchestration, `fmt.Errorf("...: %w", err)`
5. `api/controller/<resource>_controller.go` — Parse request, call usecase, return `domain.RespSuccess()`/`domain.RespError()` with proper HTTP status
6. `api/route/` — Register routes (REST: GET list, POST create, GET :id, PUT :id, DELETE :id). Protected system routes use `casbinMiddleware.CheckAPIPermission()`
7. `api/route/factory.go` — Wire Repository → Usecase → Controller using `f.db`/`f.app`/`f.timeout`

### Adding a Frontend Feature

1. `web/src/types/` — Request/response types aligned with backend domain
2. `web/src/services/<resource>Api.ts` — Axios CRUD wrappers using `apiClient`
3. `web/src/features/<feature>/` — Page index, `components/` (table, dialog, form), `hooks/` (React Query), `data/schema.ts` (Zod)
4. `web/src/routes/_authenticated/` — Route file referencing feature component

### Response Format (backend)

All API responses use `domain.Response{Code, Msg, Data}`. Code `0` = success, `1` = error. Paginated results use `domain.PagedResult[T]` with `list`, `total`, `page`, `page_size`, `total_pages`.

### Naming

- **Go files/packages**: `lower_snake` (e.g., `loginlog_repository.go`)
- **Go exports**: PascalCase; receivers: short and meaningful
- **Frontend components**: PascalCase `.tsx` (e.g., `UsersTable.tsx`)
- **Frontend utilities/hooks**: kebab-case `.ts` (e.g., `use-debounce.ts`, `handle-server-error.ts`)
- **Frontend imports**: always use `@/` alias, never relative paths beyond `./`

### Commits

Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`). Subject ≤ 72 chars. Branches: `feat/*`, `fix/*`, `chore/*`, `docs/*`.

### Quality Gates

Before committing:
- Backend: `go fmt ./...` → `go vet ./...` → `go test ./...`
- Frontend (if changed): `pnpm lint` → `pnpm format:check`
- Schema changes: `go generate ./ent`
- API changes: regenerate Swagger

### Don'ts

- No business logic in controllers or routes
- No bypassing Casbin on protected APIs
- No unnecessary third-party dependencies
- No touching unrelated files in a change
- No hardcoded menus in frontend (menus come from backend `/api/v1/resources`)
