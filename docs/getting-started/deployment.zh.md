# 部署指南

本文档介绍如何将 Shadmin 部署到生产环境，涵盖 Docker 单容器、Docker Compose 编排及反向代理配置。

## 部署策略概览

| 方式 | 适用场景 | 复杂度 |
|------|----------|--------|
| Docker 单容器 | 快速验证、小型团队 | ⭐ |
| Docker Compose | 推荐的生产部署方式 | ⭐⭐ |
| 裸机部署 | 已有基础设施、特殊需求 | ⭐⭐⭐ |

---

## 生产环境构建

### 手动构建

```bash
# 1. 构建前端
cd web && pnpm install && pnpm run build && cd ..

# 2. 构建后端（注入版本信息）
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

CGO_ENABLED=1 go build \
  -ldflags="-s -w -X cmd.version=${VERSION} -X cmd.commit=${COMMIT} -X cmd.date=${DATE}" \
  -o shadmin .
```

> **注意：** 前端必须先构建（`pnpm run build`），因为 `web/web.go` 使用 Go embed 嵌入 `web/dist/` 目录。

构建产物 `shadmin` 是一个自包含的单二进制文件，包含前端资源、后端逻辑和 Swagger 文档。

### Docker 构建

项目自带多阶段 Dockerfile：

```bash
docker build -t shadmin:latest .
```

构建过程：
1. **Stage 1（Node）**：安装前端依赖并构建 `web/dist/`
2. **Stage 2（Go）**：编译 Go 后端，嵌入前端资源
3. **Stage 3（UPX）**：压缩二进制文件，减小镜像体积
4. **Stage 4（Debian slim）**：最终运行镜像

---

## Docker 单容器部署

最简单的部署方式，使用 SQLite 数据库：

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

部署后访问 `http://your-server:55667`。

---

## 环境变量配置

生产环境必须修改以下配置（通过 `.env` 文件或环境变量）：

### 必改项

| 变量 | 说明 | 生产建议 |
|------|------|----------|
| `APP_ENV` | 运行环境 | 设为 `production` |
| `ACCESS_TOKEN_SECRET` | Access Token 签名密钥 | 使用 `openssl rand -hex 32` 生成 |
| `REFRESH_TOKEN_SECRET` | Refresh Token 签名密钥 | 使用 `openssl rand -hex 32` 生成 |
| `ADMIN_PASSWORD` | 默认管理员密码 | 使用强密码 |

### 可选项

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `:55667` | 服务端口 |
| `CONTEXT_TIMEOUT` | `60` | 请求超时（秒） |
| `ACCESS_TOKEN_EXPIRY_HOUR` | `3` | Access Token 有效期（小时） |
| `REFRESH_TOKEN_EXPIRY_HOUR` | `24` | Refresh Token 有效期（小时） |
| `DB_TYPE` | `sqlite` | 数据库类型：`sqlite` / `postgres` / `mysql` |
| `DB_DSN` | 空（SQLite 默认 `.database/data.db`） | 数据库连接串 |
| `STORAGE_TYPE` | `disk` | 存储类型：`disk` / `s3` / `minio` |
| `STORAGE_BASE_PATH` | `./uploads` | 本地存储路径 |

---

## 数据库切换

Shadmin 默认使用 SQLite，生产环境建议切换到 PostgreSQL 或 MySQL。Ent ORM 自动处理表迁移，无需手动建表。

### 切换到 PostgreSQL

```bash
DB_TYPE=postgres
DB_DSN=postgres://shadmin:your-password@localhost:5432/shadmin?sslmode=disable
```

### 切换到 MySQL

```bash
DB_TYPE=mysql
DB_DSN=shadmin:your-password@tcp(localhost:3306)/shadmin?parseTime=true&loc=Local&charset=utf8mb4
```

> **重要：** MySQL 连接串必须包含 `parseTime=true`，否则时间字段解析会失败。

首次启动时，Ent 会自动创建所有表和索引。如果从 SQLite 切换，数据不会自动迁移，需要手动导入。

---

## Docker Compose 编排

推荐的生产部署方式。根据需求选择配置方案：

### 方案一：应用 + PostgreSQL（推荐起步方案）

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

### 方案二：应用 + PostgreSQL + MinIO（完整生产方案）

如果需要对象存储（如文件上传量大、多节点部署），添加 MinIO：

```yaml
version: '3.8'

services:
  shadmin:
    build: .
    # 或使用已构建的镜像：image: shadmin:latest
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
      - "9001:9001"   # MinIO 控制台
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

启动：

```bash
docker compose up -d
```

> **MinIO 初始化：** 首次启动后需手动在 MinIO 控制台（`http://your-server:9001`）创建 `shadmin` 存储桶。

---

## MinIO 对象存储配置

将文件存储从本地磁盘切换到 MinIO / S3 兼容存储：

```bash
STORAGE_TYPE=minio
S3_ADDRESS=your-minio-host:9000
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
S3_BUCKET=shadmin
S3_TOKEN=                          # 通常留空
```

如果使用 AWS S3，将 `STORAGE_TYPE` 设为 `s3`，`S3_ADDRESS` 设为 S3 端点。

---

## Nginx 反向代理

生产环境建议使用 Nginx 反向代理，提供 HTTPS、域名和静态资源缓存。

创建 `/etc/nginx/conf.d/shadmin.conf`：

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

    # SSL 安全配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 请求大小限制（文件上传）
    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:55667;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持（如需要）
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        proxy_pass http://127.0.0.1:55667;
        proxy_cache_valid 200 30d;
        add_header Cache-Control "public, max-age=2592000";
    }

    # Swagger 文档（生产环境可选择关闭）
    # location /swagger/ {
    #     return 404;
    # }
}
```

```bash
# 验证配置
nginx -t

# 重载
nginx -s reload
```

---

## 数据持久化

确保以下目录通过 Volume 持久化：

| 路径 | 内容 | Docker Volume |
|------|------|--------------|
| `.database/` | SQLite 数据库文件 | `-v ./database:/app/.database` |
| `uploads/` | 上传的文件（仅 `STORAGE_TYPE=disk` 时） | `-v ./uploads:/app/uploads` |
| `logs/` | 应用日志 | `-v ./logs:/app/logs` |

> **提示：** 使用 PostgreSQL/MySQL 时，不需要挂载 `.database/` 目录。使用 MinIO 时，不需要挂载 `uploads/` 目录。

---

## 构建时版本注入

Shadmin 支持通过 Go 的 `-ldflags` 在编译时注入版本信息：

```bash
go build -ldflags="-s -w \
  -X cmd.version=v1.0.0 \
  -X cmd.commit=$(git rev-parse --short HEAD) \
  -X cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o shadmin .
```

| 变量 | 说明 |
|------|------|
| `cmd.version` | 版本号（如 `v1.0.0`，或 `git describe --tags` 的输出） |
| `cmd.commit` | Git commit SHA 短哈希 |
| `cmd.date` | 构建日期（ISO 8601 格式） |

启动时会打印：`starting - Version: v1.0.0, Commit: abc1234, Built: 2024-01-01T00:00:00Z`

在 CI/CD 流水线中，通常从环境变量或 Git tag 自动获取这些值。

---

## 健康检查

为 Docker 容器添加健康检查：

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:55667/api/v1/auth/login || exit 1
```

在 Docker Compose 中：

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

## 安全加固

### Token Secret

生产环境务必替换默认的 Token Secret：

```bash
# 生成强密钥
openssl rand -hex 32

# 设置环境变量
ACCESS_TOKEN_SECRET=生成的密钥1
REFRESH_TOKEN_SECRET=生成的密钥2
```

### 最小权限运行

在 Dockerfile 中添加非 root 用户：

```dockerfile
RUN useradd -r -s /bin/false shadmin
USER shadmin
```

### 防火墙

仅暴露必要端口：

```bash
# 仅允许 Nginx 访问应用端口
ufw allow 80/tcp
ufw allow 443/tcp
# 不要直接暴露 55667 端口到公网
```

### Swagger 文档

生产环境建议通过 Nginx 禁止访问 `/swagger/` 路径，或在代码中根据 `APP_ENV` 条件注册 Swagger 路由。
