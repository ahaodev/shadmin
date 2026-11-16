<div align="center">

# Shadmin

**An Enterprise-Grade Full-Stack RBAC Permission Management System Built with Go + React**

[简体中文](./README.zh.md) · English

![Shadmin Showcase](docs/images/showcase-dark-en.jpg)

[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

`Gin` · `Ent ORM` · `Casbin` · `Shadcn UI` · `TanStack Router` · `Tailwind CSS`

</div>

---

## ✨ Features

- 🏗️ **Clean Architecture** — Domain‑driven layered design (Controller → Usecase → Repository)
- 🔐 **RBAC with Casbin** — Fine‑grained role‑based access control for APIs and menus
- 🌗 **Light & Dark Themes** — Seamless theme switching with system preference detection
- 📱 **Responsive Design** — Optimized for desktop, tablet, and mobile
- 🔍 **Global Search** — Quick navigation across menus and resources
- 📊 **Dashboard** — Data visualization with charts and statistics
- 🗂️ **Dynamic Menus** — Backend‑driven menu tree with permission‑aware rendering
- ⚡ **Cacher** — Supports Redis or go-cache, flexible and optional
- 🗄️ **Multi‑Database** — SQLite (default), PostgreSQL, MySQL out of the box

## 🚀 Get Running

### Prerequisites

- Go: use the version declared in [go.mod](go.mod); SQLite requires a C compiler (`CGO_ENABLED=1`).
- Node.js and pnpm: see `engines` and the pinned `packageManager` in [frontend/package.json](frontend/package.json).

### Run

```bash
git clone https://github.com/ahaodev/shadmin.git
cd shadmin

# Build the assets embedded in the backend.
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..

# Generate the Ent code, which is not committed.
go generate ./ent
go run .
```

Open <http://localhost:55667> and sign in with `admin` / `123`. On a fresh development setup, the server creates `.env`, the SQLite database, and seed data. Configuration is described in [.env.example](.env.example); change the default credentials before deployment.

For frontend development, keep the backend running and run `pnpm dev` in `frontend/`. Open <http://localhost:5173>; Vite proxies API requests to the backend. If you change the backend port, update the proxy in [vite.config.ts](frontend/vite.config.ts).

Swagger is available at <http://localhost:55667/swagger/index.html>. To regenerate it after changing API annotations, install the version used by the [Dockerfile](Dockerfile), then run `go generate ./...`:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go generate ./...
```

For production builds, Docker, migrations and persistence, see the [deployment guide](docs/getting-started/deployment.en.md).

## 🔐 Auth & Permissions

- **Authentication**: JWT access + refresh tokens via `Authorization: Bearer <token>`
- **API Authorization**: Casbin middleware checks `(userID, path, method)` on protected routes
- **Frontend Guards**: Permission‑aware UI via the `usePermission()` hook and `auth-store` checks (`hasPermission()` / `hasRole()`)
- **Menu System**: Dynamic menus from `/api/v1/resources`, auto‑adapted to user permissions

<details>
<summary>📁 <b>Project Structure</b></summary>

```
shadmin/
├── api/            # Controllers & routes (Gin)
├── bootstrap/      # App bootstrap, DB, Casbin, seed
├── domain/         # Entities, DTOs, interfaces
├── ent/schema/     # Ent ORM schemas
├── repository/     # Data access layer
├── usecase/        # Business logic
├── internal/       # Internal utilities
├── pkg/            # Shared packages
├── frontend/            # React frontend (Vite + shadcn/ui)
│   └── src/
│       ├── routes/       # TanStack file‑based routing
│       ├── features/     # Feature modules
│       ├── services/     # API wrappers (Axios)
│       └── stores/       # Zustand state management
├── docs/           # Documentation & images
└── main.go         # Entry point
```

</details>

## 📚 Documentation

- [Development standards](.agents/skills/shadmin-dev/SKILL.md)
- [Architecture (EN)](docs/getting-started/architecture.en.md) · [架构 (中文)](docs/getting-started/architecture.zh.md)
- [Deployment (EN)](docs/getting-started/deployment.en.md) · [部署 (中文)](docs/getting-started/deployment.zh.md)
- [CLI for AI agents](cli/README.md) · [面向 AI Agent 的 CLI(中文)](cli/README.zh.md)
## 📄 License

[MIT](LICENSE)

