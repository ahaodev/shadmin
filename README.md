<div align="center">

# Shadmin

**An Enterprise-Grade Full-Stack RBAC Permission Management System Built with Go + React**

English · [简体中文](./README.zh.md)

![Shadmin Showcase](docs/images/showcase-dark-en.jpg)

[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

`Gin` · `Ent ORM` · `Casbin` · `Shadcn UI` · `TanStack Router` · `Tailwind CSS`

</div>

---

## Online demo

<https://shadmin.ahaodev.com>

- Username `admin`
- Password `admin123`

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

On a fresh development setup, the server creates `.env`, the SQLite database, and seed data. See [.env.example](.env.example) for configuration.

Once the server is running, open <http://localhost:55667> and sign in with `admin` / `admin123`.

### Development tools

For frontend development, keep the backend running and run `pnpm dev` in `frontend/`. Open <http://localhost:5173>; Vite proxies API requests to the backend. If you change the backend port, update the proxy in [vite.config.ts](frontend/vite.config.ts).

Swagger is available at <http://localhost:55667/swagger/index.html>. To regenerate it after changing API annotations, install the version used by the [Dockerfile](Dockerfile), then run `go generate ./...`:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go generate ./...
```

## 📚 Documentation

- [Architecture and permissions](docs/getting-started/architecture.en.md)
- [Deployment, migrations, and persistence](docs/getting-started/deployment.en.md)
- [Development standards](.agents/skills/shadmin-dev/SKILL.md)
- [CLI for AI agents](cli/README.md)

## 📄 License

[MIT](LICENSE)
