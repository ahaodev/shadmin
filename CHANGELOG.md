# Changelog

## [0.5.6] - 2026-10-25

### Changed

- Logging now uses the standard `log/slog` directly: `pkg.InitLogger` installs the default logger at startup (stdout plus a size-rotated `.logs/app.log`, 14 backups, local time), `APP_ENV=dev` enables debug level, and every layer calls `slog.Info/Warn/Error` with key-value fields instead of the `pkg.Log` wrapper.
- Gin access and panic-recovery logs are written to the same rotated log file.

### Fixed

- Profile update and password change now map a missing user to `ErrUserNotFound` (404) and a unique-constraint conflict to `ErrUserExists` (409) instead of a generic 500.
- Exhausting device-code generation retries now returns a 500 instead of a 400 `invalid_device_code`.

## [0.5.5] - 2026-10-24

### Added

- Added `controller.RespondError`, backed by a single error-rules table, so every controller maps usecase errors the same way; unregistered errors return a generic 500 and are only logged.
- Added `MustBindQuery` alongside `MustBindJSON`, plus `domain.ErrUserNotFound` / `domain.ErrUserExists` with Ent `NotFound` mapping in the user, profile and menu repositories.
- Added controller, API and repository tests covering error mapping, graceful startup and authorization transactions.

### Changed

- Startup and shutdown are now error-driven: `bootstrap.App`, `conf.NewEnv`, `NewEntDatabase` and `InitStorage` return errors instead of calling `log.Fatalf`/`panic`, `cmd.Run` returns an exit code so deferred cleanup always runs, and `api.Run` uses an `http.Server` with `ReadHeaderTimeout` and graceful shutdown on `SIGINT`/`SIGTERM`.
- Gin release mode is enabled when `APP_ENV=prod`.
- Token issuing is consolidated in `tokenservice`, which now owns JWT secrets and expiry; `TokenIssuer` exposes `IssuePair` / `IssuePairWithIdentity`, removing four fields and four constructor arguments from each of three usecases.
- Every repository reads through `clientFromContext` so it joins a `UnitOfWork` transaction; menu create/update/delete-tree and role `DeleteIfUnused` use `WithAuthorizationTx` instead of hand-rolled transactions.
- Login treats only `ErrUserNotFound` as invalid credentials, so a database failure no longer counts as a failed attempt or locks the account.
- `GET /system/menu` returns all matches without paging; the unused `type`, `status`, `parent_id`, `page` and `page_size` parameters are gone from the API, Swagger, frontend types and CLI.
- Adopted `slog` throughout the logger, and renamed context parameters from `c` to `ctx`.

### Security

- Removed `GetClientIP`, which trusted `X-Forwarded-For` unconditionally; client IPs now come from `c.ClientIP()` so Gin's trusted proxies apply.

### Cleanup

- Removed unused login and context utilities, unused `LoginUsecase` lookups, and info-level audit noise from usecases.

### Tooling

- Streamlined the `.agent` skill layout and updated the backend, architecture and development guides together with the generated Swagger.

### Breaking

- 5xx responses now carry a generic `Internal server error` message, and `GET /system/menu` no longer accepts `type`, `status`, `parent_id`, `page` or `page_size`.

## [0.5.4] - 2026-10-08

### Added

- Added domain contracts for transaction orchestration and authorization snapshots, with Ent-backed repository implementations.

### Changed

- Centralized repository-to-usecase construction in bootstrap; route factories now assemble controllers from prebuilt usecases.
- Restructured third-party identity login so provider SDK conversion stays at the HTTP boundary, while usecases own account binding, login logging, and one-time code exchange. Identity account creation and binding now use the shared unit-of-work transaction contract.
- Moved token, login-security, and authorization data dependencies behind domain interfaces; Casbin snapshots are now built from repository-provided authorization facts.
- Simplified dictionary CRUD safeguards and dialogs, and validated dictionary statuses consistently at the usecase boundary.
- Refined department CRUD responses and frontend date parsing, normalized root-level parent IDs, and simplified department user-reference checks.
- Updated Swagger output and English/Chinese architecture, development, and quickstart documentation to match the current implementation.

### Cleanup

- Removed unused frontend components, hooks, utilities, and legacy domain types.

### Tooling

- Updated the frontend package manager pin to pnpm 12.10.1.
