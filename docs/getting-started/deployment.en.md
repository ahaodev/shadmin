# Deployment Guide

This document covers deploying Shadmin to production, including single Docker containers, Docker Compose orchestration, and reverse proxy configuration.

## Deployment Strategy Overview

| Method | Use Case | Complexity |
|--------|----------|-----------|
| Single Docker Container | Quick validation, small teams | ⭐ |
| Docker Compose | Recommended for production | ⭐⭐ |
| Bare Metal | Existing infrastructure, special requirements | ⭐⭐⭐ |

---

## Production Build

### Manual Build

```bash
# 1. Build the frontend
cd web && pnpm install && pnpm run build && cd ..

# 2. Build the backend (with version injection)
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

CGO_ENABLED=1 go build \
  -ldflags="-s -w -X cmd.version=${VERSION} -X cmd.commit=${COMMIT} -X cmd.date=${DATE}" \
  -o shadmin .
```

> **Note:** The frontend must be built first (`pnpm run build`), because `web/web.go` uses Go embed to include `web/dist/`.

The build output `shadmin` is a self-contained single binary that includes frontend assets, backend logic, and Swagger documentation.

### Docker Build

The project includes a multi-stage Dockerfile:

```bash
docker build -t shadmin:latest .
```

Build stages:
1. **Stage 1 (Node)**: Install frontend dependencies and build `web/dist/`
2. **Stage 2 (Go)**: Compile Go backend with embedded frontend assets
3. **Stage 3 (UPX)**: Compress the binary to reduce image size
4. **Stage 4 (Debian slim)**: Final runtime image

---

## Single Docker Container Deployment

The simplest deployment method, using SQLite:

```bash
docker run -d \
  --name shadmin \
  -p 55667:55667 \
  -v ./database:/app/.database \
  -v ./uploads:/app/uploads \
  -v ./logs:/app/logs \
  -e APP_ENV=production \
  -e ACCESS_TOKEN_SECRET=your-strong-secret-here \
  -e REFRESH_TOKEN_SECRET=your-strong-refresh-secret \
  -e ADMIN_PASSWORD=your-admin-password \
  shadmin:latest
```

Access the application at `http://your-server:55667`.

---

## Environment Variable Configuration

The following settings must be changed for production (via `.env` file or environment variables):

### Required Changes

| Variable | Description | Production Recommendation |
|----------|-------------|--------------------------|
| `APP_ENV` | Runtime environment | Set to `production` |
| `ACCESS_TOKEN_SECRET` | Access token signing key | Generate with `openssl rand -hex 32` |
| `REFRESH_TOKEN_SECRET` | Refresh token signing key | Generate with `openssl rand -hex 32` |
| `ADMIN_PASSWORD` | Default admin password | Use a strong password |

### Optional Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `:55667` | Server port |
| `CONTEXT_TIMEOUT` | `60` | Request timeout (seconds) |
| `ACCESS_TOKEN_EXPIRY_HOUR` | `3` | Access token expiry (hours) |
| `REFRESH_TOKEN_EXPIRY_HOUR` | `24` | Refresh token expiry (hours) |
| `DB_TYPE` | `sqlite` | Database type: `sqlite` / `postgres` / `mysql` |
| `DB_DSN` | empty (SQLite defaults to `.database/data.db`) | Database connection string |
| `STORAGE_TYPE` | `disk` | Storage type: `disk` / `s3` / `minio` |
| `STORAGE_BASE_PATH` | `./uploads` | Local storage path |

---

## Database Switching

Shadmin defaults to SQLite. For production, switching to PostgreSQL or MySQL is recommended. Ent ORM handles table migration automatically — no manual table creation needed.

### Switch to PostgreSQL

```bash
DB_TYPE=postgres
DB_DSN=postgres://shadmin:your-password@localhost:5432/shadmin?sslmode=disable
```

### Switch to MySQL

```bash
DB_TYPE=mysql
DB_DSN=shadmin:your-password@tcp(localhost:3306)/shadmin?parseTime=true&loc=Local&charset=utf8mb4
```

> **Important:** The MySQL connection string must include `parseTime=true`, otherwise time field parsing will fail.

On first startup, Ent will automatically create all tables and indexes. When switching from SQLite, data is not automatically migrated — you'll need to import it manually.

---

## Docker Compose Orchestration

The recommended production deployment method. Choose a configuration based on your needs:

### Option 1: App + PostgreSQL (Recommended Starting Point)

```yaml
version: '3.8'

services:
  shadmin:
    build: .
    container_name: shadmin
    restart: unless-stopped
    ports:
      - "55667:55667"
    environment:
      - APP_ENV=production
      - DB_TYPE=postgres
      - DB_DSN=postgres://shadmin:shadmin_password@postgres:5432/shadmin?sslmode=disable
      - ACCESS_TOKEN_SECRET=your-strong-access-secret
      - REFRESH_TOKEN_SECRET=your-strong-refresh-secret
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your-admin-password
      - ADMIN_EMAIL=admin@example.com
      - STORAGE_TYPE=disk
      - STORAGE_BASE_PATH=/app/uploads
    volumes:
      - ./logs:/app/logs
      - ./uploads:/app/uploads
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    container_name: shadmin-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: shadmin
      POSTGRES_USER: shadmin
      POSTGRES_PASSWORD: shadmin_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U shadmin"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

### Option 2: App + PostgreSQL + MinIO (Full Production Setup)

If you need object storage (e.g., large file uploads, multi-node deployments), add MinIO:

```yaml
version: '3.8'

services:
  shadmin:
    build: .
    # Or use a pre-built image: image: shadmin:latest
    container_name: shadmin
    restart: unless-stopped
    ports:
      - "55667:55667"
    environment:
      - APP_ENV=production
      - DB_TYPE=postgres
      - DB_DSN=postgres://shadmin:shadmin_password@postgres:5432/shadmin?sslmode=disable
      - ACCESS_TOKEN_SECRET=your-strong-access-secret
      - REFRESH_TOKEN_SECRET=your-strong-refresh-secret
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your-admin-password
      - ADMIN_EMAIL=admin@example.com
      - STORAGE_TYPE=minio
      - S3_ADDRESS=minio:9000
      - S3_ACCESS_KEY=minioadmin
      - S3_SECRET_KEY=minioadmin123
      - S3_BUCKET=shadmin
    volumes:
      - ./logs:/app/logs
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    container_name: shadmin-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: shadmin
      POSTGRES_USER: shadmin
      POSTGRES_PASSWORD: shadmin_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U shadmin"]
      interval: 5s
      timeout: 5s
      retries: 5

  minio:
    image: minio/minio:latest
    container_name: shadmin-minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin123
    ports:
      - "9001:9001"   # MinIO Console
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
  minio_data:
```

Start the services:

```bash
docker compose up -d
```

> **MinIO Initialization:** After first startup, you'll need to manually create the `shadmin` bucket in the MinIO Console (`http://your-server:9001`).

---

## MinIO Object Storage Configuration

Switch file storage from local disk to MinIO / S3-compatible storage:

```bash
STORAGE_TYPE=minio
S3_ADDRESS=your-minio-host:9000
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
S3_BUCKET=shadmin
S3_TOKEN=                          # Usually leave empty
```

For AWS S3, set `STORAGE_TYPE` to `s3` and `S3_ADDRESS` to the S3 endpoint.

---

## Nginx Reverse Proxy

For production, using Nginx as a reverse proxy is recommended to provide HTTPS, domain names, and static asset caching.

Create `/etc/nginx/conf.d/shadmin.conf`:

```nginx
server {
    listen 80;
    server_name admin.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name admin.example.com;

    ssl_certificate     /etc/nginx/ssl/admin.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/admin.example.com.key;

    # SSL security settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Request size limit (file uploads)
    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:55667;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (if needed)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Static asset caching
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        proxy_pass http://127.0.0.1:55667;
        proxy_cache_valid 200 30d;
        add_header Cache-Control "public, max-age=2592000";
    }

    # Swagger docs (consider disabling in production)
    # location /swagger/ {
    #     return 404;
    # }
}
```

```bash
# Verify configuration
nginx -t

# Reload
nginx -s reload
```

---

## Data Persistence

Ensure the following directories are persisted via volumes:

| Path | Contents | Docker Volume |
|------|----------|--------------|
| `.database/` | SQLite database file | `-v ./database:/app/.database` |
| `uploads/` | Uploaded files (`STORAGE_TYPE=disk` only) | `-v ./uploads:/app/uploads` |
| `logs/` | Application logs | `-v ./logs:/app/logs` |

> **Tip:** When using PostgreSQL/MySQL, the `.database/` mount is not needed. When using MinIO, the `uploads/` mount is not needed.

---

## Build-time Version Injection

Shadmin supports injecting version information at compile time via Go's `-ldflags`:

```bash
go build -ldflags="-s -w \
  -X cmd.version=v1.0.0 \
  -X cmd.commit=$(git rev-parse --short HEAD) \
  -X cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o shadmin .
```

| Variable | Description |
|----------|-------------|
| `cmd.version` | Version number (e.g., `v1.0.0`, or output of `git describe --tags`) |
| `cmd.commit` | Git commit SHA short hash |
| `cmd.date` | Build date (ISO 8601 format) |

On startup it prints: `starting - Version: v1.0.0, Commit: abc1234, Built: 2024-01-01T00:00:00Z`

In CI/CD pipelines, these values are typically obtained automatically from environment variables or Git tags.

---

## Health Checks

Add health checks to the Docker container:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:55667/api/v1/auth/login || exit 1
```

In Docker Compose:

```yaml
services:
  shadmin:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:55667/api/v1/auth/login"]
      interval: 30s
      timeout: 5s
      retries: 3
```

---

## Security Hardening

### Token Secrets

Always replace default token secrets in production:

```bash
# Generate strong keys
openssl rand -hex 32

# Set environment variables
ACCESS_TOKEN_SECRET=generated-key-1
REFRESH_TOKEN_SECRET=generated-key-2
```

### Least Privilege Execution

Add a non-root user in the Dockerfile:

```dockerfile
RUN useradd -r -s /bin/false shadmin
USER shadmin
```

### Firewall

Only expose necessary ports:

```bash
# Only allow Nginx to access the app port
ufw allow 80/tcp
ufw allow 443/tcp
# Do not expose port 55667 directly to the public internet
```

### Swagger Documentation

In production, consider blocking access to `/swagger/` via Nginx, or conditionally registering Swagger routes based on `APP_ENV` in the code.
