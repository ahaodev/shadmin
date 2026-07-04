# Quick Start

This guide helps you get Shadmin running locally in 5 minutes.

## Prerequisites

| Tool | Minimum Version | Notes |
|------|-----------------|-------|
| Go | 1.26+           | [Download](https://go.dev/dl/) |
| Node.js | 20+             | [Download](https://nodejs.org/) |
| pnpm | 10+             | `corepack enable` (recommended — ships with Node.js). This project is pnpm-only; npm/yarn will be rejected by `preinstall`. |

## Clone the Project

```bash
git clone https://github.com/ahaodev/shadmin.git
cd shadmin
```

## Build the Frontend

The backend embeds the frontend build output via `frontend/frontend.go`, so **you must install dependencies and build the frontend first**, otherwise Go compilation will fail:

```bash
cd frontend
pnpm install      # Install frontend dependencies
pnpm run build    # Build frontend, outputs to frontend/dist/
cd ..
```

## Start the Backend

```bash
go run .
```

On first startup, Shadmin will automatically:
- Generate a `.env` config file from `.env.example`
- Create a SQLite database (`.database/data.db`)
- Run database migrations
- Initialize the default admin account
- Scan and register API resources

Once started, the server listens at `http://localhost:55667`, serving both the API and the frontend.

## Frontend Development Mode (Optional)

If you need to develop the frontend, start the frontend dev server separately:

```bash
cd frontend
pnpm dev
```

The dev server runs at `http://localhost:5173` and proxies `/api` requests to the backend at `:55667`.

## Default Admin Login

Open `http://localhost:55667` (or `http://localhost:5173` in dev mode) and log in with the default credentials:

| Field | Default |
|-------|---------|
| Username | `admin` |
| Password | `123456` |

> Default credentials are configured in `.env` (`ADMIN_USERNAME` / `ADMIN_PASSWORD`). Change them in production.

## Swagger API Documentation

After starting the backend, visit `http://localhost:55667/swagger/index.html` to view the auto-generated API docs.

## Key Configuration Overview

Core settings in the `.env` file:

```bash
# Database — defaults to SQLite, switchable to PostgreSQL or MySQL
DB_TYPE=sqlite
# PostgreSQL: postgres://user:password@localhost:5432/dbname?sslmode=disable
# MySQL: user:password@tcp(localhost:3306)/dbname?parseTime=true&loc=Local&charset=utf8mb4
DB_DSN=

# Authentication tokens
ACCESS_TOKEN_EXPIRY_MINUTE=180
REFRESH_TOKEN_EXPIRY_MINUTE=1440
ACCESS_TOKEN_SECRET=default-access-secret    # Must change in production
REFRESH_TOKEN_SECRET=default-refresh-secret  # Must change in production

# File storage — defaults to local disk, switchable to MinIO/S3
STORAGE_TYPE=disk
STORAGE_BASE_PATH=./uploads
```

See [.env.example](../../.env.example) for the full configuration reference.

## Build for Production

Make sure you've built the frontend (`pnpm install && pnpm run build`), then:

```bash
go build -o shadmin .
./shadmin
```

The frontend is embedded in the binary — no separate frontend deployment needed.

## Docker Quick Start

```bash
# Build the image
docker build -t shadmin .

# Run (default SQLite)
docker run -d --name shadmin -p 55667:55667 shadmin

# Mount volumes for persistence (recommended)
docker run -d --name shadmin \
  -p 55667:55667 \
  -v ./database:/app/database \
  -v ./uploads:/app/uploads \
  -v ./logs:/app/logs \
  shadmin
```

Visit `http://localhost:55667` after startup.

## Troubleshooting

**Port 55667 already in use**
```bash
# Change PORT in .env
PORT=:8080
```

**Go version too old**
```bash
go version  # Requires 1.26+
```

**pnpm not installed**
```bash
corepack enable     # Node.js 16.10+ ships corepack; this activates pnpm@10.34.3 via the packageManager field
```

**Frontend dependency installation fails**
```bash
cd frontend
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

## Next Steps

- [Architecture Overview](./architecture.en.md) — Understand Shadmin's layered design
- [Development Guide](./development.en.md) — Add new feature modules
- [Deployment Guide](./deployment.en.md) — Production deployment
