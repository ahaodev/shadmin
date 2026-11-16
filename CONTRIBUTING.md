# Contributing to Shadmin

This guide exists to save both sides time.

## Design first

Shadmin is a layered template, and the layers only look one way: `api` → `usecase` → `repository` → `domain`, with frameworks kept at the edges. Authorization is data, resolved once at the boundary — not conditionals scattered through handlers. Infrastructure (database, storage, cache) sits behind contracts and is chosen by configuration.

The rules are written down once:

- [`AGENTS.md`](./AGENTS.md) — the principles, in short.
- [`.agents`](./.agents) — the authoritative, step-by-step source for how features are built (backend, frontend, permissions, checks).

If your change breaks one of these — a new dependency direction, a permission hardcoded instead of registered, a new response shape — it is a design change, not a feature. Open an issue and discuss it before writing code.

## The One Rule

**You must understand your code.** If you cannot explain what your change does and how it interacts with the rest of the system, it will not be merged.

Using AI to write code is fine. Submitting AI-generated slop without understanding it is not.

If you use an agent, run it from the repository root so it picks up `AGENTS.md` and `.agents` automatically.

## Quality bar for issues

Use one of the two issue templates. Keep it short, concrete and worth reading:

- If it does not fit on one screen, it is too long.
- State the problem clearly and explain why it matters.
- Write in your own voice. Do not submit agent-generated issues; if you must include AI-written text, label it as such.
- Say so if you intend to implement the change yourself.

Minimal repros get fixed; essays get closed. Duplicates and requests that belong in your own fork or extension will be closed without much discussion. Security problems: report them privately to the maintainer instead of opening a public issue with exploit details.

## Before you open a PR

- Do not open a PR for a large design change before it has been agreed on in an issue.
- Make the smallest change that fully solves the problem, and follow the idiom already in the file you are touching.
- Do not edit `CHANGELOG.md`. Changelog entries are added by maintainers.

Enable the repository hooks once:

```bash
git config core.hooksPath .githooks
```

They run `gofmt` and the frontend `prettier`/`eslint` checks on staged files.

Then run the gates for the surfaces you changed — the same ones CI runs:

```bash
# Backend (regenerate first if Ent schemas or annotations changed, and commit the output)
go generate ./...
gofmt -l . && go vet ./... && go test -race ./...

# Frontend
cd frontend && pnpm lint && pnpm format:check && pnpm test && pnpm build

# CLI
cd cli && make vet && make test
```

The frontend must be built before the backend, because its assets are embedded in the binary.

Commits use [Conventional Commits](https://www.conventionalcommits.org/) with a subject of at most 72 characters. One logical change per PR.

## Docs

User-facing docs live in `docs/` and are bilingual: if you change documented behavior, update both `*.en.md` and `*.zh.md`. If you change a design pattern, update `.agents` and `AGENTS.md` in the same PR — they describe the same design and must not drift apart.

## Where to start

- [README](./README.md#-get-running) — get it running.
- [Architecture](./docs/getting-started/architecture.en.md) — the layering and the boundaries.
- [Development standards](./.agents/skills/shadmin-dev/SKILL.md) — workflow, backend/frontend patterns and permissions.
