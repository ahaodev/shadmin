# Architecture Overview

This document helps you understand Shadmin's overall design so you can quickly navigate the codebase, understand the request lifecycle, and extend functionality.

## Tech Stack

| Layer | Technology                                                     |
|-------|----------------------------------------------------------------|
| Backend Framework | Go 1.25 + Gin                                                  |
| Database ORM | Ent (supports SQLite / PostgreSQL / MySQL)                     |
| Authentication | JWT (access + refresh tokens)                                  |
| Authorization | Casbin RBAC (path + method based policies)                     |
| API Documentation | Swagger / OpenAPI (auto-generated via swaggo)                  |
| Logging | Logrus + file rotation                                         |
| Frontend Framework | React 19 + TypeScript                                          |
| Build Tool | Vite + SWC                                                     |
| UI Components | Shadcn UI (Radix UI + Tailwind CSS v4)                         |
| Routing | TanStack Router (file-based routing, automatic code splitting) |
| Data Layer | TanStack Query + Axios                                         |
| State Management | Zustand                                                        |
| Forms | React Hook Form + Zod                                          |

## System Architecture Diagram

```mermaid
graph TB
    User[User] --> FE[Frontend React SPA]
    FE -->|HTTP /api/v1/*| GW[Gin Router]
    GW --> MW[Middleware Chain]
    MW -->|JWT Validation| AUTH[JWT Auth]
    MW -->|Permission Check| CASBIN[Casbin RBAC]
    MW --> CTRL[Controller]
    CTRL --> UC[Usecase Layer]
    UC --> REPO[Repository Layer]
    REPO --> ENT[Ent ORM]
    ENT --> DB[(SQLite / PostgreSQL / MySQL)]
    UC --> FS[File Storage]
    FS --> DISK[Local Disk]
    FS --> MINIO[MinIO / S3]
```

## Directory Structure

```
shadmin/
├── main.go              # Entry point, calls cmd.Run()
├── cmd/                 # Application startup & version management
├── bootstarp/           # Bootstrap wiring: DB, Casbin, storage, seed data (note: typo is intentional)
├── api/
│   ├── controller/      # HTTP controllers (request parsing + response, no business logic)
│   ├── route/           # Route registration + middleware mounting + DI factory
│   └── middleware/      # JWT auth, Casbin authorization, logging
├── domain/              # Domain layer: entities, DTOs, interface contracts, error definitions, response wrappers
├── usecase/             # Usecase layer: business orchestration, validation, timeout control
├── repository/          # Repository layer: Ent data access + file storage implementations
├── ent/
│   └── schema/          # Ent database schema definitions
├── internal/            # Internal utilities: Casbin manager, token service, login security
├── pkg/                 # Shared utilities: logging, etc.
├── docs/                # Swagger generated files + architecture docs
├── web/                 # React frontend
│   ├── src/
│   │   ├── routes/      # File-based routing (TanStack Router)
│   │   ├── features/    # Feature modules (pages + components + hooks + schema)
│   │   ├── services/    # API call wrappers
│   │   ├── stores/      # Zustand state management
│   │   ├── components/  # Shared components + Shadcn UI
│   │   ├── hooks/       # Custom Hooks
│   │   ├── types/       # TypeScript type definitions
│   │   ├── lib/         # Utility functions
│   │   └── context/     # React Context Providers
│   └── web.go           # Go embed, embeds dist/ into the binary
└── .env.example         # Environment variable template
```

## Backend Layers

Shadmin's backend follows **Clean Architecture**, with dependencies flowing inward:

```
Route → Controller → Usecase → Repository → Ent/DB
                         ↑
                      Domain (interface contracts)
```

| Layer | Directory | Responsibility | Key Rules |
|-------|-----------|---------------|-----------|
| Domain | `domain/` | Entities, DTOs, Repository/UseCase interfaces, error definitions | Pure definitions, no implementation dependencies |
| Schema | `ent/schema/` | Database table structure | Must run `go generate ./ent` after changes |
| Repository | `repository/` | Ent data read/write + domain↔ent conversion | Data access only, no HTTP or business logic |
| Usecase | `usecase/` | Business orchestration, validation, timeout control | Every method uses `context.WithTimeout`, wrap errors with `%w` |
| Controller | `api/controller/` | HTTP request parsing + response formatting | Parse and call Usecase only, no business logic |
| Route | `api/route/` | Route registration, middleware mounting | RESTful style, system routes use Casbin middleware |
| Factory | `api/route/factory.go` | DI factory: assembles Repo → Usecase → Controller | Dependencies come from `f.db` / `f.app` / `f.timeout` |

### Dependency Injection

Shadmin uses the **Factory Pattern** for manual dependency injection, with no DI framework:

```go
// api/route/factory.go
func (f *ControllerFactory) CreateUserController() *controller.UserController {
    ur := repository.NewUserRepository(f.db, f.app.CasManager)
    rr := repository.NewRoleRepository(f.db)
    return &controller.UserController{
        UserUsecase: usecase.NewUserUsecase(f.db, ur, rr, f.timeout),
        Env:         f.app.Env,
    }
}
```

### Unified Response Format

All APIs use `domain.Response` for consistent responses:

```go
type Response struct {
    Code int         `json:"code"`  // 0 = success, 1 = error
    Msg  string      `json:"msg"`
    Data interface{} `json:"data"`
}

// Usage
c.JSON(http.StatusOK, domain.RespSuccess(result))
c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
```

Paginated results use `domain.PagedResult[T]`:

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "list": [...],
    "total": 100,
    "page": 1,
    "page_size": 10,
    "total_pages": 10
  }
}
```

## Request Lifecycle

Example: `GET /api/v1/system/user`

```mermaid
sequenceDiagram
    participant C as Client
    participant R as Gin Router
    participant JWT as JWT Middleware
    participant CAS as Casbin Middleware
    participant CTRL as UserController
    participant UC as UserUsecase
    participant REPO as UserRepository
    participant DB as Database

    C->>R: GET /api/v1/system/user
    R->>JWT: Validate Authorization Header
    JWT->>JWT: Parse Token, inject user context (x-user-*)
    JWT->>CAS: Pass
    CAS->>CAS: CheckPermission(userID, path, method)
    CAS->>CTRL: Pass
    CTRL->>CTRL: Parse Query params (page, page_size, search...)
    CTRL->>UC: ListUsers(ctx, filter)
    UC->>UC: context.WithTimeout + ValidateQueryParams
    UC->>REPO: Query(ctx, filter)
    REPO->>DB: Ent Query (Where + Offset + Limit)
    DB-->>REPO: Return data
    REPO-->>UC: []*domain.User
    UC-->>CTRL: *PagedResult[*User]
    CTRL-->>C: JSON { code: 0, data: {...} }
```

## Frontend Structure

### Route Organization

The frontend uses TanStack Router's **file-based routing**, where file paths map to URL paths:

```
web/src/routes/
├── __root.tsx              # Root layout (DevTools, progress bar)
├── (auth)/                 # Public route group (login, register)
│   └── sign-in.tsx
├── (errors)/               # Error pages (404, 500)
└── _authenticated/         # Protected route group
    ├── route.tsx            # Auth guard (beforeLoad checks JWT)
    └── system/
        ├── user.tsx         # → features/system/users
        ├── role.tsx         # → features/system/roles
        └── menu.tsx         # → features/system/menus
```

The `beforeLoad` hook in `_authenticated/route.tsx` checks for `accessToken` and redirects to the login page if invalid.

### Feature Modules

Each feature is encapsulated as an independent module:

```
features/system/users/
├── index.tsx            # Page entry component
├── components/          # Tables, dialogs, forms, buttons
├── hooks/               # TanStack Query hooks (useUsers, useCreateUser...)
├── data/schema.ts       # Zod runtime validation
└── lib/                 # Form schemas, utility functions
```

### Data Flow

```
API Service (Axios) → TanStack Query (cache + auto-refetch) → React Components
                                                                   ↕
                                                           Zustand Store (auth state)
```

- **API Calls**: Functions in `services/` use `apiClient` (Axios), which auto-injects the `Bearer` token
- **Data Caching**: TanStack Query manages request caching, loading states, and automatic retries
- **Global State**: `auth-store` manages user identity, tokens, and permission info
- **Response Parsing**: Use `response.data.data` to access business data (outer `.data` is Axios, inner `.data` is `domain.Response.Data`)

## Authentication Flow

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant BE as Backend
    participant DB as Database

    U->>FE: Enter username/password
    FE->>BE: POST /api/v1/auth/login
    BE->>DB: Validate credentials (bcrypt)
    BE->>BE: Generate Access Token (short-lived) + Refresh Token (long-lived)
    BE-->>FE: { accessToken, refreshToken }
    FE->>FE: Store tokens (localStorage + Cookie)
    FE->>FE: Parse JWT for basic user info
    FE->>BE: GET /api/v1/profile (with Bearer Token)
    BE-->>FE: User details
    FE->>BE: GET /api/v1/resources (with Bearer Token)
    BE-->>FE: Menus + permission list
    FE->>FE: Render sidebar + enable permission controls
```

## Permission Model

Shadmin uses **Casbin RBAC** for access control:

```
User → Role → Permission Policy → API Resource (path + method)
               ↓
           Menu Binding → Frontend sidebar + button visibility
```

**Backend**: Casbin middleware checks whether `(userID, requestPath, requestMethod)` matches a policy.
**Frontend**: `auth-store` provides `hasPermission()` / `hasRole()` / `canAccessMenu()` methods. `PermissionButton` / `PermissionGuard` components control UI visibility.

### Menu and Resource Mechanism

1. On startup, `bootstrap.InitApiResources()` scans all Gin routes and writes them to the database with IDs in `METHOD:/path` format
2. Admins bind API resources to menu items in Menu Management
3. When roles are assigned menus, Casbin policies are automatically synced
4. The frontend fetches the current user's visible menu tree and permission list via `/api/v1/resources` to dynamically render the sidebar

## Database Abstraction

Switch databases via `DB_TYPE` in `.env` — no code changes required:

| DB_TYPE | Description | DB_DSN Example |
|---------|-------------|---------------|
| `sqlite` | Default, zero config | Leave empty (auto uses `.database/data.db`) |
| `postgres` | Recommended for production | `postgres://user:pass@localhost:5432/shadmin?sslmode=disable` |
| `mysql` | Optional | `user:pass@tcp(localhost:3306)/shadmin?parseTime=true&loc=Local` |

Ent ORM automatically runs schema migrations on startup.

## File Storage Abstraction

Switch storage backends via `STORAGE_TYPE`:

| STORAGE_TYPE | Description | Key Config |
|-------------|-------------|-----------|
| `disk` | Default, local filesystem | `STORAGE_BASE_PATH=./uploads` |
| `minio` | MinIO / S3-compatible storage | `S3_ADDRESS`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` |

A unified `domain.FileRepository` interface allows transparent backend switching.

## Startup Flow

```
main.go
  → cmd.Run()
    → bootstrap.App()              # Load .env, connect DB, init Casbin/storage/Gin
    → api.SetupRoutes(app)         # Register static assets, Swagger, API routes
    → bootstrap.InitApiResources() # Scan Gin routes and write to DB
    → bootstrap.InitDefaultAdmin() # Init admin role/user, bind menus and policies
    → bootstrap.InitDictData()     # Init dictionary data
    → bootstrap.InitCasbinHooks()  # Set up Casbin rules
    → api.Run(app)                 # Listen on port :55667
```

## Frontend-Backend Collaboration

- **Production Mode**: Frontend build output (`web/dist/`) is embedded into the binary via `web/web.go` using Go embed. The backend serves both the SPA and API.
- **Development Mode**: Vite dev server runs on `:5173` and proxies `/api` requests to the backend at `:55667` via `vite.config.ts`.

## Next Steps

- [Development Guide](./development.en.md) — Add new feature modules
- [Deployment Guide](./deployment.en.md) — Production deployment
