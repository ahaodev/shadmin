# AGENTS.md

This repo is a **template for RBAC admin systems** — its value is the pattern, so extensions preserve the design below. This file carries **design intent only**; the step-by-step module recipe and layer tables live in `.github/skills/shadmin-dev/SKILL.md`, the runbook/bootstrap detail in `.github/copilot-instructions.md`. Keep it small — mechanics belong there.

## Design intent

- **Contract-first, dependencies inward.** `domain/` declares each module's entity, DTOs and `XxxRepository` / `XxxUseCase` interfaces. Controllers hold a `domain.XxxUseCase`, usecases hold a `domain.XxxRepository` (never `*ent.Client`), implementations are unexported structs returning the domain interface, wired only in `api/route/factory.go` — `route → controller → usecase → repository → ent`, one direction.
- **One responsibility per layer.** Route = registration. Middleware = cross-cutting. Controller = HTTP parsing (`ShouldBindJSON`/`Query`/`Param`) + status mapping. Usecase = business logic, validation, `context.WithTimeout`, cross-repo orchestration. Repository = persistence + domain↔ent conversion. `bootstrap/` = startup wiring. Breaking this is the main way the template degrades.
- **Explicit manual DI, no globals.** Infrastructure (DB, storage, cache, Casbin) sits behind domain interfaces and is selected by env (`DB_TYPE`, `STORAGE_TYPE`, `CACHE_TYPE`); swapping SQLite→Postgres or disk→MinIO must not touch business code.
- **RBAC is data, never code.** Authorization is `(subject, object, action)`, enforced once by `CasbinMiddleware.CheckAPIPermission()` and persisted in `casbin_rules`. Lifecycle: register a route → startup scan writes an `apiresource` row → admin UI binds it to a menu and grants the menu to a role → enforcer resyncs. So a protected API means *registering a route*, not editing permission lists. Menus come from the backend (`/api/v1/resources`), never the frontend.
- **Auth is defense-in-depth.** JWT validity → JTI blacklist → account state (`UserStateMiddleware`) → Casbin, each gate independent. Token creation/parsing stays in `internal/tokenservice` + `internal/tokenutil`.
- **One envelope, one truth.** Every response is `domain.Response{Code,Msg,Data}` (`0` = success), pages via `domain.PagedResult[T]`; paging math lives once in `domain.QueryParams.Paginate()`, and only service wrappers (`services/*Api.ts`) unwrap `response.data.data`.
- **Generated code is authoritative.** `ent/schema/` is the source of truth; run `go generate ./ent` after changes and treat `ent/` as a build artifact — never hand-edit it.
- **Config over forks.** New configurable behavior belongs in `internal/conf/env.go` with a default, not a hardcoded branch.

## Never

- Business logic in controllers or routes.
- Bypass Casbin on `/system/*`.
- Hardcode menus or permission strings in the frontend — they mirror backend `system:<resource>:<action>` exactly (`frontend/src/constants/permissions.ts`, gated through `usePermission()` → `lib/permissions.ts`).
- Leak `ent` types past the repository boundary, or change the response envelope / `domain` interfaces.
- Add a third-party dependency without strong justification.
- Put CLI settings in the repo-root `.env` (server-only; CLI config lives under `cli/`).

## Conventions

- **Errors**: wrap with `%w` and context, sentinels in `domain/`, HTTP status mapped in the controller, not deeper.
- **Commits**: Conventional Commits (`feat:`/`fix:`/`chore:`/`docs:`), subject ≤ 72 chars; branches `feat/*`, `fix/*`, `chore/*`, `docs/*`.
- **Tests**: live beside the code (`internal/`, `api/middleware/`); no frontend test runner, so verify frontend changes with `pnpm build` + `pnpm lint`.

## Commands

```bash
go run .                                       # :55667; first run creates .env, .database/data.db, migrates, seeds admin+menus, scans routes
go build -o shadmin .                          # embedded build needs frontend/dist/ (pnpm build first)
go test ./...  |  go fmt ./... && go vet ./...  # gates mirrored in .githooks/pre-commit and CI
go generate ./ent                              # after ent/schema/ changes
pnpm dev | build | lint | format:check | knip  # frontend (pnpm only)
cd cli && make build | test | lint             # CLI (separate module)
```
