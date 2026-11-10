# Agent Instructions

- Follow the root `AGENTS.md`.
- For feature work, read `.agents/skills/shadmin-dev/SKILL.md` and only the backend or frontend reference needed for the task.
- For cleanup, read `.agents/skills/cleanup-specialist/SKILL.md`.
- Load only task-relevant files. Human-facing `docs/` is not default agent context.
- Run checks for the surfaces changed:
  - Backend: `go fmt ./... && go vet ./... && go test ./...`.
  - Frontend: `cd frontend && pnpm lint && pnpm format:check && pnpm test`; build when routes or configuration change.
  - CLI: `cd cli && make vet && make test`.
- Build the frontend before building the backend; its assets are embedded in the binary.
- Use Conventional Commits with a subject of at most 72 characters.
