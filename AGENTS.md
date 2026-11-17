## Principles

**Dependencies point one way.** Declare contracts in `domain/`. Usecases depend on those contracts, not concrete repositories or infrastructure; implementations are wired in `bootstrap`. Request flow is not an import graph. Keep frameworks at the edges.

**Authorization is data, not code.** Register protected capabilities and grants through API resources, menus and roles. Do not hardcode authorization exceptions in handlers. The backend owns menus; the frontend renders what it is given.

**Security composes from independent gates.** Authentication, token validity, account state and authorization each stand alone, so no single mistake is enough to grant access.

**Every answer has one shape.** Requests, responses and pagination follow a single contract, defined once and reused everywhere.

**Infrastructure is a choice, not an assumption.** Databases, storage and caches sit behind contracts and are selected by configuration. Add configuration for a concrete deployment need; avoid speculative flags for business behavior.

**Keep it simple.** Make the smallest coherent change that fully solves the problem. Prefer direct code and one obvious path. Introduce reusable abstractions when concrete callers justify them; domain contracts define boundaries even when there is only one caller.

## Working here

- Keep changes in scope and follow the idiom of the files touched.
- Run the checks for the surfaces changed before claiming completion.
- If a design changes, update the affected principles and implementation guidance together.

## Where rules live

This file defines stable principles. `.agents/CONTRIBUTING.md` owns workflow and checks; skills and references own task-specific implementation rules. Prompt templates apply only to their matching tasks. Load only what the task needs; agent-specific entry points link back to these sources.

| Need                                       | Read                                                                   |
| ------------------------------------------ | ---------------------------------------------------------------------- |
| Workflow, checks, generated files, commits | [.agents/CONTRIBUTING.md](.agents/CONTRIBUTING.md)                     |
| Feature workflow, wiring, permissions      | [Development skill](.agents/skills/shadmin-dev/SKILL.md)               |
| Backend implementation                     | [Backend patterns](.agents/skills/shadmin-dev/references/backend.md)   |
| Frontend implementation                    | [Frontend patterns](.agents/skills/shadmin-dev/references/frontend.md) |
| Cleanup                                    | [Cleanup skill](.agents/skills/cleanup-specialist/SKILL.md)            |
| Architecture explanation                   | [Architecture](docs/getting-started/architecture.en.md)                |
