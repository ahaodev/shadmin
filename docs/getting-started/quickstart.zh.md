# 快速开始

本指南帮助你在 5 分钟内将 Shadmin 运行起来。

## 环境要求

| 工具 | 最低版本  | 说明 |
|------|-------|------|
| Go | 1.25+ | [下载](https://go.dev/dl/) |
| Node.js | 18+   | [下载](https://nodejs.org/) |
| pnpm | 9+    | `npm install -g pnpm`（也可用 npm） |

## 克隆项目

```bash
git clone https://github.com/ahaodev/shadmin.git
cd shadmin
```

## 构建前端

后端通过 `web/web.go` 将前端产物嵌入二进制文件，因此**必须先安装依赖并构建前端**，否则 Go 编译会失败：

```bash
cd web
pnpm install      # 安装前端依赖
pnpm run build    # 构建前端，输出到 web/dist/
cd ..
```

## 启动后端

```bash
go run .
```

首次启动时，Shadmin 会自动：
- 从 `.env.example` 生成 `.env` 配置文件
- 创建 SQLite 数据库（`.database/data.db`）
- 执行数据库迁移
- 初始化默认管理员账号
- 扫描并注册 API 资源

启动成功后，服务监听在 `http://localhost:55667`，同时提供 API 和前端页面。

## 前端开发模式（可选）

如果你需要开发前端，可以单独启动前端开发服务器：

```bash
cd web
pnpm dev
```

开发服务器启动在 `http://localhost:5173`，自动将 `/api` 请求代理到后端 `:55667`。

## 默认管理员登录

打开 `http://localhost:55667`（或开发模式下 `http://localhost:5173`），使用以下默认账号登录：

| 字段 | 默认值      |
|------|----------|
| 用户名 | `admin`  |
| 密码 | `123456` |

> 默认凭据在 `.env` 中配置（`ADMIN_USERNAME` / `ADMIN_PASSWORD`），请在生产环境中修改。

## 访问 Swagger API 文档

启动后端后，访问 `http://localhost:55667/swagger/index.html` 查看自动生成的 API 文档。

## 关键配置速览

`.env` 文件中的核心配置项：

```bash
# 数据库 —— 默认 SQLite，可切换为 PostgreSQL 或 MySQL
DB_TYPE=sqlite
# PostgreSQL: postgres://user:password@localhost:5432/dbname?sslmode=disable
# MySQL: user:password@tcp(localhost:3306)/dbname?parseTime=true&loc=Local&charset=utf8mb4
DB_DSN=

# 认证令牌
ACCESS_TOKEN_EXPIRY_HOUR=3
REFRESH_TOKEN_EXPIRY_HOUR=24
ACCESS_TOKEN_SECRET=default-access-secret    # 生产环境务必修改
REFRESH_TOKEN_SECRET=default-refresh-secret  # 生产环境务必修改

# 文件存储 —— 默认本地磁盘，可切换为 MinIO/S3
STORAGE_TYPE=disk
STORAGE_BASE_PATH=./uploads
```

完整配置请参考 [.env.example](../../.env.example)。

## 构建生产二进制

确保已执行过前端构建（`pnpm install && pnpm run build`），然后：

```bash
go build -o shadmin .
./shadmin
```

前端已嵌入二进制，无需额外部署前端文件。

## Docker 快速启动

```bash
# 构建镜像
docker build -t shadmin .

# 运行（默认 SQLite）
docker run -d --name shadmin -p 55667:55667 shadmin

# 挂载数据卷持久化（推荐）
docker run -d --name shadmin \
  -p 55667:55667 \
  -v ./database:/app/database \
  -v ./uploads:/app/uploads \
  -v ./logs:/app/logs \
  shadmin
```

启动后访问 `http://localhost:55667`。

## 常见问题

**端口 55667 被占用**
```bash
# 修改 .env 中的 PORT
PORT=:8080
```

**Go 版本过低**
```bash
go version  # 需要 1.25+
```

**pnpm 未安装**
```bash
npm install -g pnpm
```

**前端依赖安装失败**
```bash
cd web
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

## 下一步

- [架构概览](./architecture.zh.md) — 理解 Shadmin 的分层设计
- [开发指南](./development.zh.md) — 基于脚手架新增功能模块
- [部署指南](./deployment.zh.md) — 生产环境部署
