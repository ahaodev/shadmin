# Changelog

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
