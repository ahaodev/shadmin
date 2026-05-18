# shadmin-cli

A thin Go CLI on top of the [Shadmin](../README.md) REST API, designed to be
consumed by external **AI agents**.

- Independent Go module (`cli/`), produces a single binary `shadmin-cli`.
- Default output is **JSON** (stable, agent-friendly). `--pretty` switches to
  a tabular format for humans.
- Authentication: OAuth device authorization flow → JWT cached in `cli/.env`
  or the `SHADMIN_CONFIG` path (file mode `0600`).
- Authorization: every call reuses the logged-in user's existing RBAC. The
  CLI **cannot bypass** server-side permission checks.
- MVP is **read-only**. No `create / update / delete` commands are exposed.

## Install / Build

```bash
cd cli
make build           # → shadmin-cli
# or:
make install         # installs into $GOBIN
```

Binary is built with version metadata injected via `-ldflags`:
`shadmin-cli --version` prints `version (commit X, built Y)`.

## Configuration

CLI configuration is scoped to `cli/`. Do not add CLI-only settings to the
repository root `.env` / `.env.example`; those files are for the backend server.

Repo-local example:

- `cli/.env.example` — server URL setup (`SHADMIN_SERVER`)

`cli/.env` is generated and maintained by `shadmin-cli login` as the local token
cache. You can also copy `cli/.env.example` to `cli/.env` for repo-local server setup.

| Source                      | Priority |
|-----------------------------|----------|
| `--server` flag             | highest  |
| `SHADMIN_SERVER` env var    |          |
| `cli/.env`                  | lowest   |

Override the config path entirely with `SHADMIN_CONFIG=/some/path.env`.

## Commands (MVP)

```text
shadmin-cli
├── login            [--server URL]
├── logout
├── whoami
├── users
│   ├── list         [--page N] [--page-size N] [--keyword K]
│   └── get <id>
├── roles
│   ├── list
│   └── get <id>
├── menus
│   ├── tree
│   ├── list
│   └── get <id>
└── api-resources list
```

Global flags: `--pretty`, `--server URL`. JSON is the default output format.

## Quick start

```bash
# 1. Login with device authorization
shadmin-cli login --server http://localhost:55667
# Open the printed URL in a browser and enter the displayed user code.

# 2. Verify
shadmin-cli whoami

# 3. Query
shadmin-cli users list --page 1 --page-size 10
shadmin-cli menus tree --pretty
shadmin-cli api-resources list
```

## Exit codes

| Code | Meaning                                      |
|------|----------------------------------------------|
| `0`  | success                                      |
| `1`  | generic error                                |
| `2`  | usage error (missing arg / flag)             |
| `3`  | network error                                |
| `4`  | unauthenticated or token expired             |
| `5`  | permission denied (HTTP 403)                 |
| `6`  | not found (HTTP 404)                         |
| `7`  | server error (HTTP 5xx or invalid response)  |

The 401 path is handled transparently: the CLI auto-refreshes the access token
once before failing with exit `4`.

## Security notes

- The CLI inherits **all** RBAC permissions of the logged-in user. Treat the
  cached config as sensitive credentials.
- The token file is written with mode `0600`; do not relax it.
- Write operations are intentionally not implemented in the MVP. If you need
  them, propose them on the backend first so the change runs through Casbin.

## AI agent integration

A ready-to-use Anthropic skill is provided at
[`skill/shadmin-cli/SKILL.md`](skill/shadmin-cli/SKILL.md), with example JSON
outputs in [`skill/shadmin-cli/examples/`](skill/shadmin-cli/examples/).

## Layout

```
cli/
├── main.go
├── cmd/                # cobra commands (root, auth, resources)
├── internal/
│   ├── client/         # HTTP client, envelope decode, 401 auto-refresh
│   ├── config/         # cli/.env config file + env overrides
│   ├── output/         # JSON / pretty rendering
│   └── clierr/         # exit codes + error wrapping
├── skill/shadmin-cli/  # Anthropic SKILL.md + examples for AI agents
├── Makefile
└── README.md
```

## Testing

```bash
make test   # unit tests for config + client (httptest-based)
```

## Roadmap

The following items are explicitly **out of scope** for the MVP and tracked for
later iterations:

- Write commands (create / update / delete) gated by stricter audit.
- Backend audit field distinguishing CLI vs Web origin.
- Optional MCP server packaging on top of the same client layer.
