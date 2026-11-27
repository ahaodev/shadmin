# Agent Workflow

Follow the principles in [AGENTS.md](../AGENTS.md). For features, read the [development skill](skills/shadmin-dev/SKILL.md) and the relevant backend/frontend reference. For cleanup, read the [cleanup skill](skills/cleanup-specialist/SKILL.md). Human-facing `docs/` is not default agent context.

## Checks

Run commands from the repository root. Apply every row whose code or runtime configuration changed; documentation-only and agent-instruction changes use the documentation row.

Before backend Go checks on a fresh checkout, build the embedded frontend assets and generate Ent. Rebuild frontend assets when frontend code/configuration changes or `frontend/dist/` is absent; see the [README](../README.md#-get-running). Run `go generate ./ent` when Ent output is missing or schemas change. When API annotations change, install swag as described in the README and run `go generate ./...`, which also generates Ent.

| Surface changed                                              | Required checks                                                              |
| ------------------------------------------------------------ | ---------------------------------------------------------------------------- |
| Backend Go code or dependencies                              | Go formatting check below, `go vet ./...`, `go test -race ./...`             |
| Frontend source, dependencies or runtime/build configuration | `(cd frontend && pnpm lint && pnpm format:check && pnpm test && pnpm build)` |
| CLI Go code or dependencies                                  | Go formatting check below, `(cd cli && go vet ./... && go test -race ./...)` |
| Documentation or agent instructions                          | Check local links, changed command examples and `git diff --check`           |

For Go changes, report formatting differences as a failed check:

```bash
shadmin_unformatted=$(gofmt -l .) || exit 1
if [ -n "$shadmin_unformatted" ]; then
  printf '%s\n' "$shadmin_unformatted"
  exit 1
fi
```

After regeneration, inspect tracked output for the intended changes. If a check cannot run, report the reason and what was skipped; never report a skipped check as passed.

Pushes to `main` and PRs targeting `main` run [frontend CI](../.github/workflows/frontend.yml) and [backend CI](../.github/workflows/backend.yml) by changed paths. Backend CI uses a minimal HTML fixture for `go:embed`.

## Generated files

Do not hand-edit generated output. Update its source and regenerate it when required.

| Files                                                    | Version control policy                                                                     |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Generated Ent code under `ent/`                          | Not committed; `ent/schema/` and `ent/generate.go` are maintained source and are committed |
| `frontend/src/routeTree.gen.ts`                          | Tracked; update through Vite (`pnpm dev`) after route changes and commit the result        |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | Tracked; regenerate after API annotation changes and commit the result                     |
| `frontend/dist/`                                         | Build artifact, not committed                                                              |

The tracked route tree must remain available because `pnpm build` runs TypeScript before Vite. Do not delete it as unused code or add it to `.gitignore`.

## Commits and releases

Pushing a `v*` tag [checks and builds the frontend, tests the backend with that `dist`, then publishes the image and GitHub Release](../.github/workflows/release.yml). The image embeds the same `dist`.

Use Conventional Commits with a subject of at most 72 characters. Stage only changes belonging to the task. A commit request does not imply push, comments, issue closure or a release; follow the actions authorized in the conversation.

Contributors leave `CHANGELOG.md` to maintainers. Maintainers update it when preparing a release, using [cl.md](prompts/cl.md); ordinary task completion does not create a version section. Preserve released history. Full task wrap-up uses [wr.md](prompts/wr.md) only when requested.
