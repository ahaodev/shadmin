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

首次启动自动创建 `.env`、SQLite 数据库和种子数据。配置见 [.env.example](.env.example)。

本地登录 <http://localhost:55667>（`admin` / `admin123`）。

### 开发工具

- **前端** — 保持后端运行，在 `frontend/` 中执行 `pnpm dev`，访问 <http://localhost:5173>。如果更改了后端端口，请调整 [vite.config.ts](frontend/vite.config.ts) 中的代理地址。
- **Swagger** — 访问 <http://localhost:55667/swagger/index.html>。修改 API 注解后，执行 `go install github.com/swaggo/swag/cmd/swag@v1.16.6` 安装 `swag`，再在仓库根目录运行 `go generate ./...`。

## 📚 文档

- [架构与权限](docs/getting-started/architecture.zh.md)
- [部署、迁移与持久化](docs/getting-started/deployment.zh.md)
- [开发规范](.agents/skills/shadmin-dev/SKILL.md)
- [面向 AI Agent 的 CLI](cli/README.zh.md)

## 📄 许可证

[MIT](LICENSE)
