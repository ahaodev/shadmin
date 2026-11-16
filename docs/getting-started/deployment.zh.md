# 部署指南

本文介绍生产配置、迁移和持久化。开发环境请使用 [README](../../README.zh.md#-跑起来)。完整配置参考见 [.env.example](../../.env.example)。

## 生产配置

首次部署时，复制模板为 `.env`，编辑后再启动服务：

```bash
cp .env.example .env
chmod 600 .env
```

设置以下字段；每个密钥分别用 `openssl rand -hex 32` 生成：

```dotenv
APP_ENV=prod
ACCESS_TOKEN_SECRET=replace-with-generated-access-secret
REFRESH_TOKEN_SECRET=replace-with-generated-refresh-secret
ADMIN_PASSWORD=replace-with-a-strong-admin-password
```

生产环境拒绝内置 JWT 密钥。种子密码仅在首次创建管理员时使用，不会重置已有账号。本地密码要求至少 8 个字符，且不超过 72 UTF-8 字节。

启用 Google/GitHub 登录时，配置 provider 凭据和独立的 `IDENTITY_SESSION_SECRET`。`IDENTITY_BASE_URL` 应为后端公网 origin，`IDENTITY_REDIRECT_URL` 应为前端 `/oauth-callback` 地址。启用邮件邀请时，将 `INVITATION_ACCEPT_URL` 设为前端公网 `/accept-invitation` 地址，并配置 Resend 字段。

环境变量优先于 `.env`。修改配置后需重启服务。

## Docker Compose

[Dockerfile](../../Dockerfile) 负责构建前端、生成 Go 代码并打包单个二进制。以下示例使用 SQLite 和持久化绑定目录。在仓库根目录创建 `compose.yaml`：

```yaml
services:
  shadmin:
    build: .
    image: shadmin:local
    restart: unless-stopped
    ports:
      - "127.0.0.1:55667:55667"
    env_file: .env
    volumes:
      - ./.database:/app/.database
      - ./.uploads:/app/.uploads
      - ./.logs:/app/.logs
```

镜像使用 UID/GID 1000 运行。在 Linux 主机上，先创建绑定目录并赋予该用户写权限：

```bash
mkdir -p .database .uploads .logs
sudo chown 1000:1000 .database .uploads .logs

docker compose build
docker compose run --rm shadmin ./shadmin-runner -migrate
docker compose up -d
```

迁移成功后再启动服务。通过 `docker compose logs --tail=100 shadmin` 检查启动日志。端口仅绑定本机，供反向代理访问；远程检查且尚未配置代理时，可通过 SSH 转发访问。

## 数据库迁移

生产启动不会修改表结构，即使 `DB_AUTO_MIGRATE=true`。非生产环境默认启用增量表结构同步，可设置 `DB_AUTO_MIGRATE=false` 关闭。

`-migrate` 执行 Ent 增量表结构变更后退出，不启动服务。使用与服务进程一致的配置和数据库运行：

```bash
# Docker Compose
docker compose run --rm shadmin ./shadmin-runner -migrate

# 原生二进制
./shadmin -migrate
```

操作前备份，并在恢复的数据库副本上验证。删除或重命名列、转换已有数据需要经过审查的版本化 SQL 迁移，不能用 `-migrate` 替代。

升级流程：

1. 构建新镜像或二进制。
2. 停止应用，备份数据库、`.env` 和上传文件。SQLite 应在服务停止后备份整个 `.database/` 目录。
3. 使用新版本执行所需迁移；失败时先排查解决，再启动服务。
4. 启动新版本，验证健康接口、登录和角色访问。回退二进制不会撤销数据库变更。

## 数据库、存储与缓存选择

### PostgreSQL 或 MySQL

预先创建数据库，在 `.env` 设置 `DB_TYPE` 和 `DB_DSN`。使用外部数据库时，Docker 示例中的 `.database/` 挂载可移除。

```dotenv
# PostgreSQL
DB_TYPE=postgres
DB_DSN=postgres://shadmin:password@database-host:5432/shadmin?sslmode=require

# 或 MySQL
DB_TYPE=mysql
DB_DSN=shadmin:password@tcp(database-host:3306)/shadmin?parseTime=true&loc=Local&charset=utf8mb4
```

数据库地址必须能从容器内访问；容器内的 `localhost` 指向容器自身。MySQL 需要 `parseTime=true`。切换数据库引擎不会自动迁移数据，需单独导入，并对目标数据库执行迁移步骤。

### 文件存储

磁盘存储默认使用 `.uploads/`。修改 `STORAGE_BASE_PATH` 时，同时调整挂载路径。使用 MinIO/S3 时，设置 `STORAGE_TYPE=minio` 和 [.env.example](../../.env.example) 中的 `S3_*` 字段。`S3_ADDRESS` 是不带 URL 协议头的 `host:port` 地址，`S3_SECURE` 决定是否使用 HTTPS。使用适合部署环境的 bucket 和凭据。

| 容器路径         | 持久化内容                         |
| ---------------- | ---------------------------------- |
| `/app/.database` | SQLite 数据库，含 journal/WAL 文件 |
| `/app/.uploads`  | 使用默认磁盘存储时的上传文件       |
| `/app/.logs`     | 轮转应用日志                       |

`.env` 和备份保存在镜像之外。使用外部服务时，单独保护并备份数据库和对象存储。

### 多实例

所有实例必须连接同一权威数据库，使用一致的 Token 密钥。上传文件使用共享对象存储；设置 `CACHE_TYPE=redis` 和 `REDIS_*` 字段，共享用户状态、Captcha 和 JWT 黑名单缓存。

每个系统请求先读取数据库授权 generation，再使用本地 Casbin 快照。generation 变化时重建快照，读取或重建失败返回 HTTP 503。因此，每个受保护系统请求会增加一次数据库读取。本地提交触发器和 `AUTHZ_SYNC_POLL_INTERVAL_SECONDS` 轮询仅负责快照预热与恢复，Redis 不会刷新其他实例的 Enforcer。

## 反向代理与健康检查

由反向代理终止 HTTPS，将请求转发到应用的本机端口。在已有 Nginx TLS server 块中，可配置：

```nginx
location / {
    proxy_pass http://127.0.0.1:55667;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 50m;
}
```

按实际需求设置上传限制。如果 Swagger 不应公开，在代理层拦截 `/swagger/`。修改后检查并重载代理配置。

在宿主机或监控服务中执行健康检查：

```bash
curl --fail http://127.0.0.1:55667/api/v1/health
```

该接口仅说明 HTTP 服务能响应，不检测数据库就绪状态或授权功能。运行镜像未安装 `curl`，容器内检查需使用包含探测客户端的派生镜像。登录接口使用 POST，不能作为 GET 健康检查地址。

## 原生二进制

按 [README](../../README.zh.md#-跑起来) 构建前端并生成 Ent 后，在仓库根目录执行：

```bash
CGO_ENABLED=1 go build -o shadmin .
```

在目标主机的工作目录放置 `.env`，赋予服务用户所配置数据与日志目录的写权限，执行 `./shadmin -migrate`，再通过进程管理器启动 `./shadmin`。二进制嵌入了前端和 Swagger，但仍需要运行时库和持久化数据。迁移、备份和反向代理步骤与 Docker 部署一致。
