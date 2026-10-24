---
name: shadmin-dev
description: Shadmin feature-development standards (backend Go/Gin/Ent + frontend React/TS). Use for any new feature, CRUD module, API route/controller/usecase/repository, Ent schema change, permission, frontend page/table/form/hook, or full-stack work in this repo — even if the user doesn't say "feature development".
---

# Shadmin Feature Development

Request flow: `Route → [JWT MW → Casbin MW] → Controller → Usecase → Repository → Ent/DB`. Frontend: `Route file → Page → TanStack Query hook → API service → apiClient`.

Dependencies point one way: each layer imports only the layer beneath it, via interfaces declared in `domain/`.

| Layer | Path | Does | Must not |
|---|---|---|---|
| Domain | `domain/` | Entities, DTOs, Repository/UseCase interfaces, sentinel errors, `Resp*` helpers | import anything outward |
| Schema | `ent/schema/` | DB schema (`go generate ./ent`) | — |
| Repository | `repository/` | Ent access, ent↔domain conversion | HTTP / permission logic |
| Usecase | `usecase/` | Business rules, validation, timeout | touch `*ent.Client`, `gin`, or `internal/` concrete types |
| Controller | `api/controller/` | Parse HTTP, call usecase, map errors→status, Swagger | business logic |
| Middleware / Route | `api/middleware/`, `api/route/` | JWT, Casbin, route registration | — |
| Factory | `api/route/factory.go` | Build controllers from `f.uc` / `f.app` | import `ent` or `repository` |
| Bootstrap | `bootstrap/` | Init, DB, Casbin, seeds, repository→usecase wiring (`usecases.go`) | — |

Frontend: `types/` (DTO types) → `services/` (Axios wrappers, re-exported in `services/index.ts`) → `features/<module>/<resource>/` → `routes/_authenticated/`. Also `stores/auth-store.ts` (Zustand), `constants/permissions.ts`, `hooks/`, `lib/`.

## Workflow

### 1. Clarify (before coding)
Entities/fields; endpoints (path, method, shapes); permission strings (`system:<resource>:<action>`); pages/tables/forms/dialogs.

### 2. Touched files
```
# Backend, in order
domain/<resource>.go
ent/schema/<resource>.go          → go generate ./ent
repository/<resource>_repository.go
usecase/<resource>_usecase.go
api/controller/<resource>_controller.go
api/route/system_routes.go        (setup<Resource>Management) + call it in protected.go setupSystemRoutes
bootstrap/usecases.go             (field + construction)
api/route/factory.go              (Create<Resource>Controller)
bootstrap/user_init.go            (seed menu/button + API resources, if shipped by default)

# Frontend, in order
types/<resource>.ts
services/<resource>Api.ts          (+ export in services/index.ts)
constants/permissions.ts
features/system/<resources>/{index.tsx, components/, hooks/, data/schema.ts}
routes/_authenticated/system/<resource>.tsx
```

### 3. Backend — read `references/backend.md`
- IDs `xid.New().String()`; update DTOs use pointer fields; list filters embed `domain.QueryParams` and call `Paginate()`.
- Responses: `domain.RespSuccess(data)` (code 0) / `domain.RespError(err)` (code 1); lists use `domain.NewPagedResult`.
- Every usecase method: `context.WithTimeout` + `defer cancel()`; errors wrapped `%w`; sentinel errors mapped to status in the controller.
- Protected system routes: `group.Use(casbinMiddleware.CheckAPIPermission())`.

### 4. Frontend — read `references/frontend.md`
- Read `response.data.data` (Axios `.data` → `domain.Response.Data`); services convert date strings to `Date`.
- Mutations via `useCrudMutation`; table URL state via `useTableUrlState`; dialogs via a provider; gate UI with `usePermission()`.

### 5. Permissions
Two layers, linked by Role → Menu → API resources:
- **API:** Casbin checks `(userID, path, method)`. On startup `bootstrap.InitApiResources` scans Gin routes into `api_resources` with IDs `METHOD:/api/v1/path` (e.g. `GET:/api/v1/system/department/:id`).
- **UI:** permission strings on `button` menus, delivered to the frontend by `/api/v1/resources` into `auth-store`. Admins bypass.

To protect a new feature: register routes (auto-scanned) → add keys to `constants/permissions.ts` → create menu/button entries bound to the API resources (admin UI, or seed in `bootstrap/user_init.go`) and assign to roles. A menu's `path` must equal the route file's URL.

**Authorization writes.** Ent user/role/menu/API-resource relations are the source of truth; each process builds an in-memory Casbin snapshot from them. Any write that changes those edges must commit together with the `authz_states.generation` bump — use `repository.WithAuthorizationTx` (nested calls bump once). Never mutate the live Enforcer or use Casbin AutoSave. A stale snapshot fails closed (HTTP 503). Recovery polling: `AUTHZ_SYNC_POLL_INTERVAL_SECONDS` (default 3600). Keep existing `g/p` semantics (active users/roles, public resources excluded, `admin` wildcard) unless a reviewed change says otherwise.

### 6. Verify
```bash
go generate ./ent            # schema changed
go generate ./...            # Swagger annotations changed (needs swag CLI)
go fmt ./... && go vet ./... && go test ./...
cd frontend && pnpm lint && pnpm format:check && pnpm build   # build if routes/config changed
```
Full command list: `.agent/CONTRIBUTING.md`.

## Boundaries
- No business logic in controllers; no HTTP/permission logic in repositories.
- No bypassing Casbin on protected APIs; no new globals (use `f.uc` / `f.app`).
- No inline API calls in React — go through `services/`. No direct localStorage for auth — use `useAuthStore`.
- Don't edit `components/ui/` (shadcn). Don't hardcode menus — they come from `/api/v1/resources`.
- No unnecessary dependencies; touch only files relevant to the feature.

## References
- `references/backend.md`, `references/frontend.md` — patterns and conventions (read the one you need).
- `docs/getting-started/{architecture,development}.{en,zh}.md` — walkthrough and a full CRUD example.
