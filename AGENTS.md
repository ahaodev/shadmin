# AGENTS.md

Shadmin is a **template for RBAC admin systems**. Its value is the pattern, not the feature set — so when extending it, preserve the shape below rather than reinvent it.

This file holds **direction only**: the ideas that stay true as the code changes. Recipes, layer tables, file paths and commands belong to the skill, which is where they can stay correct.

## Principles

**Dependencies point one way.** Each module declares its contracts first; every layer knows only the layer beneath it, and frameworks stay at the edges. When an inner layer starts reaching outward, the design has broken — that, not a missing feature, is what degrades this template. Change the pattern on purpose, never by accident.

**Authorization is data, not code.** Permissions are facts in a database, resolved once at the boundary — not conditionals scattered through handlers. Adding a protected capability should mean *registering* it, never editing a permission list. The same instinct applies to menus: the backend owns them, the frontend only renders what it is given.

**Security composes from independent gates.** Authentication, token validity, account state and authorization each stand alone, so no single mistake is enough to grant access.

**Every answer has one shape.** Requests, responses and pagination follow a single contract, defined once and reused everywhere. Uniformity here is a feature, not a style preference.

**Infrastructure is a choice, not an assumption.** Databases, storage and caches sit behind contracts and are selected by configuration, so swapping one out never reaches business logic. New behavior should be configurable by default, not hardcoded.

**Less is more.** The smallest change that fully answers the problem wins. Prefer deleting a branch over adding a flag, one obvious path over a configurable many, fewer files over more. Every line added is a line someone must read, test and maintain, so new code has to earn its place — if it can be simpler without losing behavior, it is not finished.

**Keep it simple (KISS).** Design for the problem in front of you, not the one you imagine later. Code that reads top-to-bottom beats clever indirection, and abstractions arrive when a second concrete caller proves them, never before. When two designs both work, choose the one with fewer moving parts.

## Working here

- Keep changes minimal and in-scope; follow the idiom already in the file you are touching.
- Verify before you claim done. The gates are cheap — run them.
- This file and the skill describe one design. If you change the design, update both.

## Specifics live in the skill

`.agent/skills/shadmin-dev/` is the authoritative, step-by-step source for how things are actually built — read it before writing feature code. Everything under `.agent/` is the single source of truth; each agent's own directory (`.github/`, `.pi/`, `.claude/`) links back to it.

| Need | Read |
| --- | --- |
| Workflow, layer responsibilities, boundaries, permissions | `.agent/skills/shadmin-dev/SKILL.md` |
| Backend patterns — domain, Ent, repository, usecase, controller, routes | `.agent/skills/shadmin-dev/references/backend.md` |
| Frontend patterns — types, services, features, hooks, forms, routes | `.agent/skills/shadmin-dev/references/frontend.md` |
| Build/test/lint commands, bootstrap, config, conventions | `.agent/CONTRIBUTING.md` |
| Architecture walkthrough | `docs/getting-started/architecture.en.md` |
