<div align="center">

# Shadmin

**基于 Go + React 构建的企业级全栈 RBAC 权限管理系统**

简体中文 · [English](./README.md)

![Shadmin 展示](docs/images/showcase-dark.jpg)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
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
- 🗄️ **多数据库支持** — 开箱即用支持 SQLite（默认）、PostgreSQL、MySQL

## 📁 项目结构

```
shadmin/
├── api/            # 控制器和路由（Gin）
├── bootstarp/      # 应用引导、数据库、Casbin、种子数据
├── domain/         # 实体、DTO、接口定义
├── ent/schema/     # Ent ORM 数据模型
├── repository/     # 数据访问层
├── usecase/        # 业务逻辑层
├── internal/       # 内部工具
├── pkg/            # 公共包
├── web/            # React 前端（Vite + shadcn/ui）
│   └── src/
│       ├── routes/       # TanStack 文件路由
│       ├── features/     # 功能模块
│       ├── services/     # API 封装（Axios）
│       └── stores/       # Zustand 状态管理
├── docs/           # 文档和图片
└── main.go         # 入口文件
```

## 🚀 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+ & pnpm（或 npm）

### 开发环境

```bash
# 克隆仓库
git clone https://github.com/ahaodev/shadmin.git
cd shadmin

# 启动后端（监听 :55667，.env 缺失时自动生成）
go run .

# 启动前端（新终端窗口）
cd web && pnpm install && pnpm dev
```

### 生产构建

```bash
# 构建前端
cd web && pnpm build

# 构建后端（内嵌 web/dist/）
go build -o shadmin .

# 运行
./shadmin
```

### Makefile 快捷命令

| 命令 | 说明 |
|------|------|
| `make run` | 仅启动后端 |
| `make dev` | 启动后端 + 前端开发服务器 |
| `make build` | 构建后端和前端 |
| `make check` | 运行所有代码检查和测试 |
| `make hooks` | 启用 pre‑commit Git 钩子 |
| `make help` | 列出所有可用目标 |

## 🔐 认证与权限

- **认证**：JWT 访问令牌 + 刷新令牌，通过 `Authorization: Bearer <token>` 传递
- **API 鉴权**：Casbin 中间件对受保护路由检查 `(userID, path, method)`
- **前端守卫**：权限感知组件（`PermissionButton`、`PermissionGuard`）
- **菜单系统**：通过 `/api/v1/resources` 获取动态菜单树，自动适配用户权限

## 🛠️ 开发指南

```bash
# Ent 模型代码生成（修改 ent/schema/* 后执行）
go generate ./ent

# Swagger 文档生成（需安装 swag CLI）
go install github.com/swaggo/swag/cmd/swag@latest
go generate

# 运行测试
go test ./...

# 前端代码检查和格式化
cd web && pnpm lint && pnpm format:check
```

## 📚 文档

- [架构 (中文)](docs/getting-started/architecture.zh.md) · [Architecture (EN)](docs/getting-started/architecture.en.md)
- [快速开始 (中文)](docs/getting-started/quickstart.zh.md) · [Quick Start (EN)](docs/getting-started/quickstart.en.md)
- [开发指南 (中文)](docs/getting-started/development.zh.md) · [Development (EN)](docs/getting-started/development.en.md)
- [部署 (中文)](docs/getting-started/deployment.zh.md) · [Deployment (EN)](docs/getting-started/deployment.en.md)

