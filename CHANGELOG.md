# Changelog

## [0.5.9] - 2026-11-22

### Changed

- Startup now applies additive Ent schema changes in every environment, including `prod`, whenever `DB_AUTO_MIGRATE` is true; `-migrate` remains for explicit pre-start runs.
- `.env` is created only when absent and contains built-in defaults instead of environment overrides; an existing file is read as-is, and environment variables still override it at runtime.

### Security

- Generated `.env` files are created with `0600` instead of `0644`.

### Breaking

- Production startup migrates the schema by default, reversing the previous no-migration startup: set `DB_AUTO_MIGRATE=false` and run `shadmin -migrate` before startup to keep manual control, and back up the database first.

## [0.5.8] - 2026-11-21

### Added

- Added `STORAGE_TYPE=s3` for AWS S3 alongside `minio`, using a pre-created bucket, the region from `S3_REGION` or the AWS SDK configuration, and the default credential chain when explicit keys are absent.
- Added `S3_REGION`, which defaults to `us-east-1` for MinIO and to the resolved AWS region for S3.

### Changed

- Object storage now uses the AWS SDK for Go v2 instead of `minio-go`, reaching MinIO through a path-style endpoint built from `S3_ADDRESS` and `S3_SECURE` and rejecting addresses with a mismatched scheme, path, query or credentials at startup.
- Storage validation now covers both object backends: `S3_BUCKET` is always required, explicit S3 credentials must be set as an access/secret pair, `S3_TOKEN` requires both, and unknown `STORAGE_TYPE` values are reported as `disk`, `s3` or `minio`.
- Object listings are read one folder level per page with folder entries synthesized from common prefixes, URL-decoded keys and unquoted ETags.
- Uploads verify the declared size before committing a single PUT or completing a multipart upload.

### Tooling

- Refreshed Go and frontend dependencies, including the AWS SDK v2, the MySQL driver, `resend`, `x/crypto`, Atlas, axios, TanStack Query, ESLint, Vite and Prettier.

## [0.5.7] - 2026-11-17

### Added

- Added `-migrate`, which applies additive schema changes and exits, plus `DB_AUTO_MIGRATE` for non-production startup.
- Added `S3_SECURE` (default `true`) for TLS MinIO connections.
- Added a per-request authorization generation check, and unified JSON responses for panics, unknown routes and disallowed methods.
- Added session-scoped frontend token storage with cross-tab sync, request cancellation and single-flight refresh.

### Changed

- `/system` authorization now mounts once on the route group and no longer skips paths by prefix.
- Refresh-token rotation is single-winner through an atomic blacklist `Consume`; a losing concurrent refresh gets 401.
- Menu and permission state moved into the auth store, and menu load failures surface a retry screen instead of an empty sidebar.
- Sidebar groups expand for the active path and flatten nested links when collapsed.
- Enforced an 8-character/72-byte password policy on user create/update, password change, invitation acceptance and production admin seeding.
- Login-log lists default to 10 rows per page, and department updates verify the new parent exists.
- Upgraded to Go 1.27.3 and refreshed frontend dependencies.

### Fixed

- Role rename and duplicate dictionary labels now return 409 instead of 500.
- Unique-constraint conflicts map by specific index instead of any constraint error.
- Credential checks run before hashing, so a missing user or wrong current password no longer returns 500.
- Concurrent refresh no longer wedges the session: refreshes serialize, stale requests cancel, and logout happens only on 400/401/403.
- `useCrudMutation` no longer rethrows after its error toast.

### Security

- Production startup rejects default JWT and identity secrets, and requires a policy-compliant admin password when seeding.
- Tokens are no longer mirrored into readable cookies, and legacy keys and cookies are purged at startup.
- Removed the path exemption that let listed paths bypass a mounted authorization gate.
- Removed hardcoded S3 credentials from defaults, stopped logging DSNs, and generalized connection errors.
- Refresh tokens must carry an unexpired `jti` with an available blacklist.

### Cleanup

- Removed duplicated status and source constants and unused helpers, and added an architecture test enforcing dependency direction.

### Tooling

- CI derives Go from `go.mod`, runs CLI and frontend test suites, verifies Swagger drift, pins `swag` v1.16.6, and gates releases on PostgreSQL/MySQL/Redis integration tests.
- Added a pre-commit hook running gofmt, Prettier and ESLint on staged files.
- Pinned Docker toolchains and added `VERSION`/`COMMIT` build args for `linux/amd64` releases.

### Breaking

- Production startup no longer creates or alters the schema, and `WithDropIndex`/`WithDropColumn` were dropped: run `shadmin -migrate` (additive only) or reviewed SQL before deploying.
- The 8-character/72-byte password policy is enforced on writes, so shorter or longer passwords are rejected.
- Only the first concurrent refresh succeeds; the loser receives 401 and must sign in again.
- Token refresh requires Web Locks, that is HTTPS or localhost; insecure contexts and older browsers are asked to sign in again.

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
