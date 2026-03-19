---
name: shadmin-dev
description: Apply Shadmin feature-development standards (backend Go/Gin/Ent + frontend React/TS). Use when adding/modifying features, CRUD modules, API routes/controllers/usecases/repositories, Ent schemas, web pages/routes, React components, TanStack hooks, or any full-stack work in this project. Trigger whenever the user mentions new features, backend changes, frontend changes, database schema changes, permissions, UI pages, tables, forms, or API endpoints — even if they don't explicitly say "feature development."
---

# Shadmin Feature Development

Guide full-stack feature development through Shadmin's clean architecture, producing code that compiles, passes lint/tests, and follows established patterns. Most features require both backend and frontend changes — this skill covers the end-to-end workflow.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  Frontend (React 19 + TypeScript + Vite)                            │
│  Route File → Page Component → TanStack Query Hook → API Service   │
│       ↕ Zustand (auth-store) ↕ Permission checks                   │
├─────────────────────────────────────────────────────────────────────┤
│  HTTP (Axios apiClient ← Bearer Token injection)                   │
├─────────────────────────────────────────────────────────────────────┤
│  Backend (Go + Gin + Ent ORM)                                      │
│  Route → [JWT MW → Casbin MW] → Controller → Usecase → Repository  │
│       ↕ Domain (contracts, DTOs, errors) ↕ Ent (DB, migrations)    │
└─────────────────────────────────────────────────────────────────────┘
```

**Backend layers** — each has exactly one responsibility:

| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| Domain | `domain/` | Entity structs, DTOs, Repository/UseCase interfaces, errors, response helpers |
| Schema | `ent/schema/` | DB schema → run `go generate ./ent` after changes |
| Repository | `repository/` | Data access via Ent, domain↔ent conversion, pagination |
| Usecase | `usecase/` | Business logic, validation, `context.WithTimeout` |
| Controller | `api/controller/` | HTTP parsing only, Swagger annotations, status code mapping |
| Route | `api/route/` | Route registration, middleware wiring |
| Factory | `api/route/factory.go` | DI: repo → usecase → controller construction |
| Bootstrap | `bootstarp/` | App init, DB, Casbin, seeds (directory name typo is intentional) |

**Frontend layers:**

| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| Types | `web/src/types/` | TypeScript interfaces matching backend DTOs |
| Services | `web/src/services/` | Axios API wrappers, date parsing |
| Features | `web/src/features/` | Page components, tables, dialogs, forms, hooks |
| Routes | `web/src/routes/` | TanStack Router file-based routing |
| Stores | `web/src/stores/` | Zustand state (auth, permissions) |
| Constants | `web/src/constants/` | Permission strings, enums |

## Full-Stack Development Workflow

### Step 1: Clarify Scope (before writing code)

State explicitly:
- What entities/fields are involved
- API endpoints: path, method, request/response shapes
- Whether Casbin permission checks are needed
- Frontend: pages, tables, forms, dialogs
- Permission strings (e.g., `system:project:add`)

### Step 2: List All Touched Files

Group by layer — this catches missing pieces early:

```
# Backend (implement in this order)
domain/<resource>.go
ent/schema/<resource>.go
repository/<resource>_repository.go
usecase/<resource>_usecase.go
api/controller/<resource>_controller.go
api/route/<resource>_routes.go (or modify system_routes.go)
api/route/factory.go

# Frontend (implement in this order)
web/src/types/<resource>.ts
web/src/services/<resource>Api.ts
web/src/features/<module>/<resource>/components/*-provider.tsx
web/src/features/<module>/<resource>/hooks/use-<resource>.ts
web/src/features/<module>/<resource>/components/*-columns.tsx
web/src/features/<module>/<resource>/components/*-table.tsx
web/src/features/<module>/<resource>/components/*-form-dialog.tsx
web/src/features/<module>/<resource>/components/*-dialogs.tsx
web/src/features/<module>/<resource>/components/*-primary-buttons.tsx
web/src/features/<module>/<resource>/data/schema.ts
web/src/features/<module>/<resource>/index.tsx
web/src/routes/_authenticated/<module>/<resource>.tsx
web/src/constants/permissions.ts (add new permission keys)
```

### Step 3: Implement Backend

Follow the layer order strictly — each layer depends on the one above.

**Read `references/backend.md` for complete code templates and patterns.**

Quick reference for key conventions:
- **IDs**: `xid.New().String()` in Ent schema `DefaultFunc`
- **Partial updates**: pointer fields in `Update*Request` (`*string`)
- **Pagination**: embed `domain.QueryParams`, call `domain.ValidateQueryParams()`
- **Response**: `domain.RespSuccess(data)` (code=0) / `domain.RespError(msg)` (code=1)
- **Usecase**: every method starts with `context.WithTimeout` + `defer cancel()`
- **Errors**: sentinel errors in domain, `%w` wrapping, map to HTTP status in controller
- **Factory**: repo → usecase → controller, dependencies from `f.db`, `f.app`, `f.timeout`
- **Routes**: protected system routes use `casbinMiddleware.CheckAPIPermission()`

### Step 4: Implement Frontend

Follow the order: types → service → feature module → route file.

**Read `references/frontend.md` for complete code templates and patterns.**

Quick reference for key conventions:
- **API response**: `response.data.data` (outer `.data` = Axios, inner `.data` = `domain.Response.Data`)
- **Date parsing**: API service converts string dates to `Date` objects
- **Query params**: `URLSearchParams` construction, snake_case to match backend
- **Table state**: `useTableUrlState` hook syncs pagination/filters with URL
- **Dialog state**: string-based via context provider (`open === 'add' | 'edit' | 'delete'`)
- **Permissions**: `usePermission()` hook, `PERMISSIONS.SYSTEM.RESOURCE.ACTION` constants
- **Toast**: `sonner` for success/error notifications
- **Forms**: React Hook Form + Zod, single hook handles create/edit
- **Route file**: Zod schema validates URL search params with `.catch()` defaults

### Step 5: Wire Permissions

Shadmin uses a **dual-layer permission model**:

```
Backend (API access):    Casbin checks (userID, path, method)
Frontend (UI visibility): Permission strings like "system:project:add"
```

These are linked through the **Role → Menu → API Resources** binding:
1. Backend auto-scans routes into API resources on startup (`bootstrap.InitApiResources`)
2. API resource IDs are deterministic: `METHOD:/api/v1/path` (e.g., `GET:/api/v1/system/project`)
3. Admin assigns menus to roles, each menu binds to API resources
4. Frontend fetches permissions from `/api/v1/resources` and stores in Zustand

**To add permissions for a new feature:**
1. Backend: routes auto-register as API resources on restart
2. Frontend: add permission constants in `web/src/constants/permissions.ts`
3. Admin panel: create menu entries, bind API resources, assign to roles

### Step 6: Generate & Verify

```bash
# Backend
go generate ./ent           # If schema changed
go fmt ./... && go vet ./... # Format + static analysis
go test ./...                # Run tests
swag init -g main.go --output ./docs  # If Swagger annotations changed

# Frontend (from web/)
pnpm lint                   # ESLint
pnpm format:check           # Prettier
pnpm build                  # Recommended if routes/build config changed
```

## Key Response Format

```go
// Success (HTTP 200/201)
type Response struct {
    Code int         `json:"code"`    // 0 = success
    Msg  string      `json:"msg"`     // "OK"
    Data interface{} `json:"data"`    // payload
}

// Error (HTTP 400/404/500)
// Code = 1, Msg = error description, Data = nil

// Paginated response (in Data field)
type PagedResult[T any] struct {
    List       []T `json:"list"`
    Total      int `json:"total"`
    Page       int `json:"page"`
    PageSize   int `json:"page_size"`
    TotalPages int `json:"total_pages"`
}
```

## Boundaries

Things to never do:
- **No business logic in controllers** — controllers parse HTTP, call usecase, return response
- **No HTTP/permission logic in repositories** — repositories do data access only
- **No bypassing Casbin** on protected APIs
- **No new globals** — use factory's `f.db`, `f.app`, `f.timeout`
- **No inline API calls in React** — all API access goes through `services/` wrappers
- **No direct localStorage for auth** — use `useAuthStore`
- **No editing `components/ui/`** — shadcn-generated primitives
- **No hardcoded menus** — menus come from backend `/api/v1/resources`
- **No unnecessary dependencies** — frontend or backend
- **Minimal changes** — only touch files relevant to the feature

## Reference Files

For detailed code templates and implementation patterns, read these as needed:

- **`references/backend.md`** — Complete Go/Gin/Ent code templates for domain, schema, repository, usecase, controller, routes, and factory. Read when implementing backend features.
- **`references/frontend.md`** — Complete React/TypeScript code templates for types, API services, feature modules, hooks, tables, forms, dialogs, routes, and permissions. Read when implementing frontend features.

## Further Documentation

The `docs/getting-started/` directory contains comprehensive guides:
- `quickstart.zh.md` / `quickstart.en.md` — Quick start guide
- `architecture.zh.md` / `architecture.en.md` — Architecture deep-dive
- `development.zh.md` / `development.en.md` — Full CRUD walkthrough with example
- `deployment.zh.md` / `deployment.en.md` — Production deployment guide
