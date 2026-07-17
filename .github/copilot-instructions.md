# Copilot Instructions — Shadmin

Shadmin is a full-stack RBAC admin dashboard with three main surfaces:
- Go backend (`main.go`, `api/`, `domain/`, `repository/`, `usecase/`, `ent/`)
- React 19 frontend (`frontend/`)
- Thin Go CLI (`cli/`) for read-only agent workflows

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

### Frontend (frontend/)

```bash
cd frontend
pnpm install                    # Install deps (pnpm only — npm/yarn rejected by preinstall hook)
pnpm dev                        # Dev server on :5173, proxies /api to :55667
pnpm build                      # Type-check + Vite build → frontend/dist/
pnpm lint                       # ESLint
pnpm format:check               # Prettier check
pnpm format                     # Prettier auto-fix
pnpm knip                       # Unused dependency check
```

> **Important**: `frontend/dist/` is embedded into the Go binary via `frontend/frontend.go`. A frontend build is required before `go build`.

### CLI (cli/)

```bash
cd cli
make build                      # Build shadmin-cli
make install                    # Install into $GOBIN
make test                       # CLI unit tests
```

### Pre-commit hook

`.githooks/pre-commit` runs in order: `gofmt -s -w .` → `go vet ./...` → `go test ./...` → frontend lint+format check. The frontend step is **skipped** if `frontend/node_modules` is not installed or `pnpm` is unavailable. CI in `.github/workflows/ci.yml` runs the same gates on a fresh runner.

## Bootstrap Runbook

The first `go run .` does five things — errors in the first 30 seconds are almost always one of these:

1. **Generate `.env`** from `.env.example` if missing.
2. **Create SQLite DB** at `.database/data.db` (switch via `DB_TYPE` + `DB_DSN`).
3. **Run Ent migrations** (auto on startup).
4. **Seed default admin** (`admin` / `123`, overridable in `.env`).
5. **Scan Gin routes** → persist to `apiresource` table so the RBAC admin UI has the full route inventory.

## Architecture

### Backend — Clean Architecture Layers

```
main.go → cmd.Run() → bootstrap.App() → api.SetupRoutes() → api.Run()
```

Request flow: **Route → Middleware (JWT + Casbin) → Controller → Usecase → Repository → Ent/DB**

| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| Entry | `cmd/` | Startup orchestration (`cmd.Run()`), version flag |
| Domain | `domain/` | Entities, DTOs, Repository/UseCase interfaces, errors, `RespSuccess()`/`RespError()` response helpers |
| Schema | `ent/schema/` | Ent ORM schema definitions (run `go generate ./ent` after changes) |
| Repository | `repository/` | Ent data access + domain↔ent model conversion; disk/minio file storage impls |
| Usecase | `usecase/` | Business logic with `context.WithTimeout`, error wrapping (`%w`) |
| Controller | `api/controller/` | HTTP parsing only (`ShouldBindJSON`/`Query`/`Param`), Swagger annotations, no business logic |
| Middleware | `api/middleware/` | `JwtAuthMiddleware`, `CasbinMiddleware.CheckAPIPermission()`, request logging |
| Route | `api/route/` | `public.go` (auth, health) + `protected.go` (system/*); middleware wiring |
| Factory | `api/route/factory.go` | DI: creates Repository → Usecase → Controller chains |
| Internal | `internal/` | Casbin manager+adapter, token service, login security (3-strike lockout), 1h sync scheduler |
| Bootstrap | `bootstrap/` | App init, DB, Casbin, storage, seed data. Note: some older docs spell this `bootstarp/` — the typo does not exist on disk |
| Shared | `pkg/` | Cross-package utilities (logging, etc.) |

### Frontend — Feature-Based Structure

```
frontend/src/
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

### CLI — Read-Only Agent Client

- Separate Go module under `cli/`
- JSON is the default output; `--pretty` switches to a tabular view
- Auth uses OAuth device authorization and caches the JWT in `cli/.env` or `SHADMIN_CONFIG`
- All requests reuse server-side RBAC; the CLI cannot bypass permissions
- MVP commands are read-only (`login`, `whoami`, `users`, `roles`, `menus`, `api-resources`)

### Auth & Permissions

- **Authentication**: JWT access + refresh tokens. Middleware extracts claims into Gin context (`x-user-*` keys).
- **Authorization**: Casbin checks `(userID, path, method)` on `/api/v1/system/*` routes via `CheckAPIPermission()` middleware.
- **Frontend**: `auth-store` exposes `hasPermission()`, `hasRole()`, `canAccessMenu()`. Use `PermissionButton`/`PermissionGuard` for gated UI.
- **Login security**: 3 failed attempts → 1-minute lockout (`internal/login_security.go`).

### Database & Storage

- Default SQLite (`.database/data.db`). Set `DB_TYPE=postgres|mysql` + `DB_DSN` for others.
- Auto-migration on startup.
- File storage: `STORAGE_TYPE=disk|minio` with abstract interface in `domain/file.go`.

| `DB_TYPE` | Example `DB_DSN` |
|-----------|------------------|
| `sqlite` (default) | leave empty |
| `postgres` | `postgres://user:pass@localhost:5432/shadmin?sslmode=disable` |
| `mysql` | `user:pass@tcp(localhost:3306)/shadmin?parseTime=true&loc=Local` |

| `STORAGE_TYPE` | Required env vars |
|----------------|-------------------|
| `disk` (default) | `STORAGE_BASE_PATH` (default `./uploads`) |
| `minio` | `S3_ADDRESS`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_TOKEN` (optional) |

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

1. `frontend/src/types/` — Request/response types aligned with backend domain
2. `frontend/src/services/<resource>Api.ts` — Axios CRUD wrappers using `apiClient`
3. `frontend/src/features/<feature>/` — Page index, `components/` (table, dialog, form), `hooks/` (TanStack Query), `data/schema.ts` (Zod)
4. `frontend/src/routes/_authenticated/` — Route file referencing feature component; `routeTree.gen.ts` regenerates on dev/build
5. `frontend/src/constants/permissions.ts` — Add permission strings matching `system:<resource>:<action>`; bind menu + API resource to role via admin UI (or `bootstrap/admin_init.go` for seed data)

### Response Format (backend)

All API responses use `domain.Response{Code, Msg, Data}`. Code `0` = success, `1` = error. Paginated results use `domain.PagedResult[T]` with `list`, `total`, `page`, `page_size`, `total_pages`.

### CLI Configuration

- CLI-only settings belong under `cli/` (`cli/.env.example`, `cli/.env`, or `SHADMIN_CONFIG`)
- Do not add CLI settings to the repository root `.env` files; those are for the backend server
- `cli/.env` is managed by `shadmin-cli login` and stores the local token cache
- `shadmin-cli` is read-only in the MVP; new write commands should be designed with backend RBAC/Casbin first

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
- CLI (if changed): `cd cli && make test`
- Schema changes: `go generate ./ent`
- API changes: regenerate Swagger

### Don'ts

- No business logic in controllers or routes
- No bypassing Casbin on protected APIs
- No unnecessary third-party dependencies
- No touching unrelated files in a change
- No hardcoded menus in frontend (menus come from backend `/api/v1/resources`)

## Key Reference Docs

- `docs/getting-started/architecture.en.md` — full architecture walkthrough with diagrams
- `docs/getting-started/development.en.md` — end-to-end "add a new module" tutorial
- `docs/getting-started/deployment.en.md` — production deployment (incl. Docker)
- `.github/skills/shadmin-dev/SKILL.md` — authoritative step-by-step feature-dev recipe
- `cli/README.md` — CLI tool documentation
