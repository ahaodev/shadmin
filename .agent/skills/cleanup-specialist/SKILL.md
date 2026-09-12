---
name: cleanup-specialist
description: Safe cleanup of Shadmin's Go backend, React frontend, Go CLI, and docs — remove dead code, consolidate duplication, and improve maintainability without changing behavior or adding features. Use when asked to simplify, deduplicate, remove dead code, tidy up, refactor for maintainability, or align naming and conventions.
---

# Shadmin Cleanup Specialist

Safe cleanup of Shadmin (Go/Gin/Ent backend, React 19 + TypeScript frontend, thin Go CLI). Simplify safely: remove dead code, consolidate duplication, and improve maintainability without changing behavior. Do not add features.

## Scope

**When a specific file or directory is mentioned:**
- Focus only on cleaning up the specified file(s) or directory
- Read outside the target only to confirm usages before deleting or merging
- Don't make changes outside the specified scope

**When no specific target is provided:**
- Sweep the repo for cleanup opportunities
- Prioritize the most impactful first: dead code, duplicated logic, then docs

## Surfaces and scope

| Surface | Paths |
|---------|-------|
| Backend (Go) | `main.go`, `cmd/`, `bootstrap/`, `api/`, `domain/`, `repository/`, `usecase/`, `ent/`, `internal/`, `pkg/` |
| Frontend (React 19 + TS) | `frontend/src/` |
| CLI (Go) | `cli/` |
| Docs | `docs/`, `README.md`, `README.zh.md`, `AGENTS.md`, `CLAUDE.md`, `.agent/*.md` |

**Never edit (generated or owned by tooling):**
- Generated code: `ent/*.go` (except `ent/schema/`), `frontend/src/routeTree.gen.ts`, `docs/docs.go`, `docs/swagger.*`, `frontend/dist/`
- shadcn primitives: `frontend/src/components/ui/`

**Frozen contracts — cleanup must not change:**
- API routes/paths. Casbin API resource IDs are deterministic (`METHOD:/api/v1/path`) and persisted in the `apiresource` table; renaming a route or path breaks role bindings.
- Permission strings (`system:<resource>:<action>`) and Casbin middleware checks.
- Response shape: `domain.Response{Code, Msg, Data}` with code `0` = success, `1` = error; `domain.PagedResult[T]` fields.
- Layer separation (route → controller → usecase → repository). Similar-looking code across layers is architecture, not duplication.
- Public CLI output contract (JSON by default, `--pretty`).

## Cleanup responsibilities

**Go backend:**
- Remove unused functions, variables, imports, unreachable branches, commented-out code, and leftover debug prints
- Consolidate duplicated domain↔ent converters, pagination (`domain.QueryParams.Paginate()`), filter parsing, and HTTP status/error mapping
- Simplify over-nested logic; keep `context.WithTimeout` + `defer cancel()` and `fmt.Errorf("...: %w", err)` wrapping intact
- Align naming with conventions: `lower_snake` files, PascalCase exports, short receivers

**React frontend:**
- Remove unused files, exports, and dependencies (use `pnpm knip` as a signal, then verify)
- Consolidate repeated TanStack Query keys, `useTableUrlState` wiring, dialog state handling, and Zod schemas
- Normalize API access: everything goes through `services/` and returns `response.data.data`; no inline API calls
- Replace duplicated permission checks with the existing `usePermission()` hook and `PERMISSIONS` constants
- Keep imports on the `@/` alias; component files PascalCase `.tsx`, utilities/hooks kebab-case `.ts`

**CLI:**
- Remove dead flags/helpers, keep JSON-default + `--pretty` output behavior, config stays under `cli/`

**Documentation:**
- Delete stale/redundant sections and boilerplate comments
- Consolidate duplicated content across `README.md`, `README.zh.md`, `AGENTS.md`, `CLAUDE.md`, and `docs/getting-started/*.{en,zh}.md` — keep each fact in one place and cross-reference
- Fix broken links and outdated commands/field names

## Rules

- Prefer the smallest diff that removes the mess; no drive-by refactors
- No new dependencies (backend or frontend)
- No hardcoded menus in React — menus come from backend `/api/v1/resources`
- Don't touch unrelated files; don't reformat files you didn't otherwise change
- One improvement at a time, and re-verify after each removal

## Verification (required)

Run the gates for every surface you touched and report exact results:

```bash
# Backend
gofmt -s -l .            # should list nothing
go vet ./...
go test ./...

# Frontend (if frontend/ changed)
cd frontend && pnpm lint && pnpm format:check
# pnpm knip              # when hunting unused exports/deps

# CLI (if cli/ changed)
cd cli && make test

# Only if you edited ent/schema/
go generate ./ent
```

If a gate can't run (e.g., `node_modules` missing or `pnpm` unavailable), say so explicitly — never claim a check passed when it was skipped.

## Workflow

1. **Inventory** — list the dead code / duplication you found and where
2. **Plan** — state the exact files and intended edits before changing anything
3. **Change** — apply one cleanup at a time, in scope
4. **Verify** — run the gates above; if behavior could be affected, compare before/after
5. **Report** — list files changed, what was removed/merged, verification results, and residual risks

Focus on cleaning up existing code, not adding features. Work on both code (`.go`, `.ts`, `.tsx`) and documentation (`.md`) files.
