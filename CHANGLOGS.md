# Changelog

## 2026-06-26 v0.4.4

- Synced remaining `frontend/` rename leftovers (`.githooks/pre-commit`, `.github/workflows/ci.yml`, `.gitignore`, `docs/swagger.{json,yaml}`, `frontend/.env.example`) for the v0.4.3 directory change.
- Refactored frontend structure: collapsed `components/layout/data/dynamic-sidebar-data.ts` and `services/sidebarService.ts` into `hooks/use-sidebar-data.ts`; deleted `services/resourceApi.ts` and unused `features/system/{users,dicts}/data/{data,schema}.ts`; introduced `lib/permissions.ts` and slimmed `usePermission`; trimmed `services/menu-service.ts` and aligned `types/{user,dict}.ts`.
- Fixed sidebar not refreshing on permission-only changes: `use-sidebar-data` now memoizes `auth.permissions` into its user-key so `setPermissions` triggers a re-fetch.

## 2026-06-25 v0.4.3

- Renamed `web/` directory to `frontend/` across the repo (Dockerfile, CI workflow, pre-commit hook, all docs, Go embed module name updated from `package web` to `package frontend`)
- Regenerated `frontend/src/routeTree.gen.ts` after the rename
- Updated `.dockerignore`, `.gitignore`, `Dockerfile` paths to match
- Updated `api/route/route.go` import path to `shadmin/frontend`
- Synced `README.md`, `README.zh.md`, `cli/README.md`, and `docs/getting-started/*.md` (en + zh) for the new directory
- Synced `.github/skills/shadmin-dev/SKILL.md` and `.github/copilot-instructions.md` for the new directory

## 2026-05-18 v2.0.0

- Added CLI device auth
- Added login slider CAPTCHA
- Added department management
- By default, the frontend build is registered via Go embed to simplify development and deployment