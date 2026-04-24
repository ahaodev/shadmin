---
name: shadmin-cli
description: |
  Use this skill when the user asks to query Shadmin admin platform resources
  (users, roles, menus, registered API resources) from a terminal — for example
  "list shadmin users", "show shadmin role detail", "dump shadmin menu tree",
  "what API endpoints does shadmin expose". The skill calls the `shadmin-cli`
  binary, which wraps the Shadmin REST API and inherits the logged-in user's
  RBAC permissions. Do NOT use this skill for write operations (create / update
  / delete) — the MVP exposes read-only commands only.
---

# shadmin-cli skill

`shadmin-cli` is a thin Go CLI on top of the Shadmin REST API. It is designed
to be driven by external AI agents.

- **Authentication**: username + password login → JWT cached locally
  (`~/.shadmin/config.yaml`, mode 0600).
- **Authorization**: every request reuses the logged-in user's existing RBAC.
  The CLI cannot bypass server-side permission checks.
- **Output**: JSON by default (stable, machine-readable). Pass `--pretty` for
  human-readable tables.
- **Exit codes**: `0` ok · `1` generic · `2` usage · `3` network ·
  `4` unauthenticated / token expired · `5` permission denied (403) ·
  `6` not found · `7` server error.

## Prerequisites

1. The `shadmin-cli` binary is on `$PATH`.
2. The user has run `shadmin-cli login` at least once on this machine.
   - To check: run `shadmin-cli whoami`. Exit code `4` means not logged in.
3. The Shadmin server is reachable (default `http://localhost:55667`).

If the user is not logged in, ask for the server URL and username, then run:

```bash
shadmin-cli login --server <URL> -u <USERNAME>
# password prompted on TTY, or pipe via:
echo -n "$PASSWORD" | shadmin-cli login --server <URL> -u <USERNAME> --password-stdin
```

## Command reference (MVP, read-only)

| Command                                    | Purpose                                  |
|--------------------------------------------|------------------------------------------|
| `shadmin-cli login [--server URL] [-u U]`  | Cache JWT locally                        |
| `shadmin-cli logout`                       | Clear local tokens                       |
| `shadmin-cli whoami`                       | Current user profile                     |
| `shadmin-cli users list [--page --page-size --keyword]` | List users (paginated)      |
| `shadmin-cli users get <id>`               | Get user by id                           |
| `shadmin-cli roles list`                   | List roles                               |
| `shadmin-cli roles get <id>`               | Get role by id                           |
| `shadmin-cli menus tree`                   | Menu tree (UI nav structure)             |
| `shadmin-cli menus list`                   | Flat menu list                           |
| `shadmin-cli menus get <id>`               | Get menu by id                           |
| `shadmin-cli api-resources list`           | All API endpoints registered by backend  |

Global flags:
- `--server URL` overrides the saved server URL for this invocation.
- `--json` (default) / `--pretty` switch output format.
- `SHADMIN_SERVER` and `SHADMIN_TOKEN` env vars override config.

## Response envelope

The CLI strips Shadmin's `{code, msg, data}` envelope and prints the inner
`data` directly. List endpoints return either:

```json
{ "list": [...], "total": N, "page": N, "page_size": N, "total_pages": N }
```

…or a plain array (e.g. `roles list`). `api-resources list` uses `items` instead
of `list`. Inspect the JSON shape before parsing.

## Error handling guidance

- **Exit 4** → token expired or missing. Ask the user to re-run
  `shadmin-cli login` and retry.
- **Exit 5** → the logged-in user lacks RBAC permission for that API. Do not
  retry; report the permission gap to the user.
- **Exit 3** → network failure. Verify the server URL and retry once.
- **Exit 6** → the requested id does not exist. Surface the error verbatim.

## Safety

- Write operations (`create / update / delete`) are NOT exposed in this MVP.
  If the user asks for one, refuse and explain.
- The CLI uses the same permissions as the logged-in user. Do not assume an
  agent context implies elevated access.

## Examples

See `examples/` for real JSON outputs from each MVP command.
