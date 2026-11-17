# Contributing to Shadmin

This guide exists to save both sides time.

## Design first

Shadmin is a layered template. Requests flow through controllers, usecases and repositories, while code depends on contracts in `domain/`; usecases never import concrete repositories or infrastructure. `bootstrap` wires the implementations. Authorization is data resolved at the boundary, and infrastructure is selected by configuration. See [AGENTS.md](./AGENTS.md) for the principles.

The rules are written down once:

- [`AGENTS.md`](./AGENTS.md) — the principles, in short.
- [Agent workflow](./.agents/CONTRIBUTING.md) — checks, generated files and commit/release rules.
- [Development skill](./.agents/skills/shadmin-dev/SKILL.md) — implementation workflow and references.

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
- Contributors do not edit `CHANGELOG.md`. Maintainers summarize changes when preparing a release; ordinary task completion does not create a version section.

Enable the repository hooks once:

```bash
git config core.hooksPath .githooks
```

They run `gofmt` and the frontend `prettier`/`eslint` checks on staged files.

Run the [checks for the surfaces changed](./.agents/CONTRIBUTING.md#checks). That file also defines regeneration, tracked output and frontend build prerequisites. Keep one logical change per PR; use the shared [commit conventions](./.agents/CONTRIBUTING.md#commits-and-releases).

## Docs

User-facing docs live in `docs/` and are bilingual: if you change documented behavior, update both `*.en.md` and `*.zh.md`. If a design changes, update its affected `.agents` guidance and any affected principles in `AGENTS.md` in the same PR.

## Where to start

- [README](./README.md#-get-running) — get it running.
- [Architecture](./docs/getting-started/architecture.en.md) — the layering and the boundaries.
- [Development standards](./.agents/skills/shadmin-dev/SKILL.md) — workflow, backend/frontend patterns and permissions.
