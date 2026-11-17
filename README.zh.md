<div align="center">

# Shadmin

**基于 Go + React 构建的企业级全栈 RBAC 权限管理系统**

简体中文 · [English](./README.md)

![Shadmin 展示](docs/images/showcase-dark.jpg)

[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

`Gin` · `Ent ORM` · `Casbin` · `Shadcn UI` · `TanStack Router` · `Tailwind CSS`

</div>

---

## ✨ 特性

- 🏗️ **整洁架构** — 领域驱动的分层设计（Controller → Usecase → Repository）
- 🔐 **Casbin RBAC 权限管理** — 细粒度的基于角色的 API 和菜单访问控制
- 🌗 **亮色 & 暗色主题** — 无缝切换，支持系统偏好检测
- 📱 **响应式设计** — 适配桌面端、平板和移动端
- 🔍 **全局搜索** — 快速导航菜单和资源
- 📊 **数据仪表板** — 图表和统计数据可视化
- 🗂️ **动态菜单管理** — 后端驱动的菜单树，权限感知渲染
- ⚡ **缓存管理** — 支持 Redis 或 go-cache，灵活可选
- 🗄️ **多数据库支持** — 开箱即用支持 SQLite（默认）、PostgreSQL、MySQL

## 🚀 跑起来

### 环境要求

- Go：使用 [go.mod](go.mod) 声明的版本；SQLite 需要 C 编译器（`CGO_ENABLED=1`）。
- Node.js 和 pnpm：见 [frontend/package.json](frontend/package.json) 的 `engines` 和固定的 `packageManager` 版本。

### 运行

```bash
git clone https://github.com/ahaodev/shadmin.git
cd shadmin

# 构建嵌入后端的前端资源。
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..

# 生成未提交到版本控制的 Ent 代码。
go generate ./ent
go run .
```

打开 <http://localhost:55667>，使用 `admin` / `123` 登录。全新开发环境下，服务会创建 `.env`、SQLite 数据库和种子数据。配置说明见 [.env.example](.env.example)；部署前请修改默认凭据。

开发前端时，保持后端运行，在 `frontend/` 中执行 `pnpm dev`，访问 <http://localhost:5173>。Vite 会将 API 请求代理到后端；修改后端端口时，同时调整 [vite.config.ts](frontend/vite.config.ts) 中的代理地址。

Swagger 地址为 <http://localhost:55667/swagger/index.html>。修改 API 注解后，安装与 [Dockerfile](Dockerfile) 一致的工具版本，再执行 `go generate ./...`：

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go generate ./...
```

生产构建、Docker、迁移和持久化见[部署指南](docs/getting-started/deployment.zh.md)。

## 🔐 认证与权限

- **认证**：JWT 访问令牌 + 刷新令牌，通过 `Authorization: Bearer <token>` 传递
- **API 鉴权**：Casbin 中间件对受保护路由检查 `(userID, path, method)`
- **前端守卫**：通过 `usePermission()` hook 与 `auth-store` 的 `hasPermission()` / `hasRole()` 判定控制 UI 显隐
- **菜单系统**：通过 `/api/v1/resources` 获取动态菜单树，自动适配用户权限

<details>
<summary>📁 <b>项目结构</b></summary>

```
shadmin/
├── api/            # 控制器和路由（Gin）
├── bootstrap/      # 应用引导、数据库、Casbin、种子数据
├── domain/         # 实体、DTO、接口定义
├── ent/schema/     # Ent ORM 数据模型
├── repository/     # 数据访问层
├── usecase/        # 业务逻辑层
├── internal/       # 内部工具
├── pkg/            # 公共包
├── frontend/            # React 前端（Vite + shadcn/ui）
│   └── src/
│       ├── routes/       # TanStack 文件路由
│       ├── features/     # 功能模块
│       ├── services/     # API 封装（Axios）
│       └── stores/       # Zustand 状态管理
├── docs/           # 文档和图片
└── main.go         # 入口文件
```

</details>

## 📚 文档

- [开发规范](.agents/skills/shadmin-dev/SKILL.md)
- [架构 (中文)](docs/getting-started/architecture.zh.md) · [Architecture (EN)](docs/getting-started/architecture.en.md)
- [部署 (中文)](docs/getting-started/deployment.zh.md) · [Deployment (EN)](docs/getting-started/deployment.en.md)
- [面向 AI Agent 的 CLI(中文)](cli/README.zh.md) · [CLI for AI agents（EN)](cli/README.md)

## 📄 许可证

[MIT](LICENSE)
