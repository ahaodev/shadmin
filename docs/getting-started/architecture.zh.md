# 架构概览

本文帮助你理解 Shadmin 的整体设计，以便快速定位代码、理解请求链路、扩展新功能。

## 技术栈

| 层 | 技术                                    |
|----|---------------------------------------|
| 后端框架 | Go 1.25 + Gin                         |
| 数据库 ORM | Ent（支持 SQLite / PostgreSQL / MySQL）   |
| 认证 | JWT（access + refresh token）           |
| 授权 | Casbin RBAC（基于路径 + 方法的策略）             |
| API 文档 | Swagger / OpenAPI（swaggo 自动生成）        |
| 日志 | Logrus + 文件轮转                         |
| 前端框架 | React 19 + TypeScript                 |
| 构建工具 | Vite + SWC                            |
| UI 组件 | Shadcn UI（Radix UI + Tailwind CSS v4） |
| 路由 | TanStack Router（文件路由，自动代码分割）          |
| 数据层 | TanStack Query + Axios                |
| 状态管理 | Zustand                               |
| 表单 | React Hook Form + Zod                 |

## 系统架构图

```mermaid
graph TB
    User[用户] --> FE[前端 React SPA]
    FE -->|HTTP /api/v1/*| GW[Gin 路由层]
    GW --> MW[中间件链]
    MW -->|JWT 校验| AUTH[JWT Auth]
    MW -->|权限校验| CASBIN[Casbin RBAC]
    MW --> CTRL[Controller 控制器]
    CTRL --> UC[Usecase 用例层]
    UC --> REPO[Repository 仓储层]
    REPO --> ENT[Ent ORM]
    ENT --> DB[(SQLite / PostgreSQL / MySQL)]
    UC --> FS[文件存储]
    FS --> DISK[本地磁盘]
    FS --> MINIO[MinIO / S3]
```

## 目录结构速查

```
shadmin/
├── main.go              # 入口，调用 cmd.Run()
├── cmd/                 # 应用启动与版本管理
├── bootstarp/           # 启动装配：DB、Casbin、存储、种子数据（注意拼写）
├── api/
│   ├── controller/      # HTTP 控制器（请求解析 + 响应，无业务逻辑）
│   ├── route/           # 路由注册 + 中间件挂载 + DI 工厂
│   └── middleware/      # JWT 认证、Casbin 授权、日志
├── domain/              # 领域层：实体、DTO、接口契约、错误定义、响应包装
├── usecase/             # 用例层：业务编排、校验、超时控制
├── repository/          # 仓储层：Ent 数据访问 + 文件存储实现
├── ent/
│   └── schema/          # Ent 数据库 schema 定义
├── internal/            # 内部工具：Casbin 管理器、Token 服务、登录安全
├── pkg/                 # 公共工具：日志等
├── docs/                # Swagger 生成文件 + 架构文档
├── web/                 # React 前端
│   ├── src/
│   │   ├── routes/      # 文件路由（TanStack Router）
│   │   ├── features/    # 功能模块（页面 + 组件 + hooks + schema）
│   │   ├── services/    # API 调用封装
│   │   ├── stores/      # Zustand 状态管理
│   │   ├── components/  # 通用组件 + Shadcn UI
│   │   ├── hooks/       # 自定义 Hooks
│   │   ├── types/       # TypeScript 类型定义
│   │   ├── lib/         # 工具函数
│   │   └── context/     # React Context Providers
│   └── web.go           # Go embed，将 dist/ 嵌入二进制
└── .env.example         # 环境变量模板
```

## 后端分层

Shadmin 后端采用 **Clean Architecture**，从外到内依赖关系为：

```
Route → Controller → Usecase → Repository → Ent/DB
                         ↑
                      Domain（接口契约）
```

| 层 | 目录 | 职责 | 关键规则 |
|----|------|------|---------|
| Domain | `domain/` | 实体、DTO、Repository/UseCase 接口、错误定义 | 纯定义，不依赖任何实现 |
| Schema | `ent/schema/` | 数据库表结构 | 修改后必须 `go generate ./ent` |
| Repository | `repository/` | Ent 数据读写 + domain↔ent 转换 | 仅做数据访问，不含 HTTP 或业务逻辑 |
| Usecase | `usecase/` | 业务编排、校验、超时控制 | 每个方法 `context.WithTimeout`，错误用 `%w` 包装 |
| Controller | `api/controller/` | HTTP 请求解析 + 响应返回 | 仅做解析和调用 Usecase，不含业务逻辑 |
| Route | `api/route/` | 路由注册、中间件挂载 | REST 风格，系统路由用 Casbin 中间件 |
| Factory | `api/route/factory.go` | DI 工厂：组装 Repo → Usecase → Controller | 依赖来自 `f.db` / `f.app` / `f.timeout` |

### 依赖注入

Shadmin 使用 **工厂模式** 进行手动依赖注入，没有 DI 框架：

```go
// api/route/factory.go
func (f *ControllerFactory) CreateUserController() *controller.UserController {
    ur := repository.NewUserRepository(f.db, f.app.CasManager)
    rr := repository.NewRoleRepository(f.db)
    return &controller.UserController{
        UserUsecase: usecase.NewUserUsecase(f.db, ur, rr, f.timeout),
        Env:         f.app.Env,
    }
}
```

### 统一响应格式

所有 API 使用 `domain.Response` 统一返回：

```go
type Response struct {
    Code int         `json:"code"`  // 0 = 成功, 1 = 错误
    Msg  string      `json:"msg"`
    Data interface{} `json:"data"`
}

// 使用方式
c.JSON(http.StatusOK, domain.RespSuccess(result))
c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
```

分页结果使用 `domain.PagedResult[T]`：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "list": [...],
    "total": 100,
    "page": 1,
    "page_size": 10,
    "total_pages": 10
  }
}
```

## 请求处理链路

以 `GET /api/v1/system/user` 为例：

```mermaid
sequenceDiagram
    participant C as 客户端
    participant R as Gin 路由
    participant JWT as JWT 中间件
    participant CAS as Casbin 中间件
    participant CTRL as UserController
    participant UC as UserUsecase
    participant REPO as UserRepository
    participant DB as 数据库

    C->>R: GET /api/v1/system/user
    R->>JWT: 校验 Authorization Header
    JWT->>JWT: 解析 Token，注入用户上下文 (x-user-*)
    JWT->>CAS: 通过
    CAS->>CAS: CheckPermission(userID, path, method)
    CAS->>CTRL: 通过
    CTRL->>CTRL: 解析 Query 参数 (page, page_size, search...)
    CTRL->>UC: ListUsers(ctx, filter)
    UC->>UC: context.WithTimeout + ValidateQueryParams
    UC->>REPO: Query(ctx, filter)
    REPO->>DB: Ent Query (Where + Offset + Limit)
    DB-->>REPO: 返回数据
    REPO-->>UC: []*domain.User
    UC-->>CTRL: *PagedResult[*User]
    CTRL-->>C: JSON { code: 0, data: {...} }
```

## 前端结构

### 路由组织

前端使用 TanStack Router 的 **文件路由**，路由定义即文件路径：

```
web/src/routes/
├── __root.tsx              # 根布局（DevTools、进度条）
├── (auth)/                 # 公开路由组（登录、注册）
│   └── sign-in.tsx
├── (errors)/               # 错误页面（404、500）
└── _authenticated/         # 受保护路由组
    ├── route.tsx            # 认证守卫（beforeLoad 校验 JWT）
    └── system/
        ├── user.tsx         # → features/system/users
        ├── role.tsx         # → features/system/roles
        └── menu.tsx         # → features/system/menus
```

`_authenticated/route.tsx` 中的 `beforeLoad` 会检查 `accessToken`，无效则重定向至登录页。

### 功能模块

每个功能封装为独立模块：

```
features/system/users/
├── index.tsx            # 页面入口组件
├── components/          # 表格、弹窗、表单、按钮
├── hooks/               # TanStack Query hooks (useUsers, useCreateUser...)
├── data/schema.ts       # Zod 运行时校验
└── lib/                 # 表单 schema、工具函数
```

### 数据流

```
API Service (Axios) → TanStack Query (缓存 + 自动重取) → React 组件
                                                            ↕
                                                    Zustand Store (认证状态)
```

- **API 调用**：`services/` 下的函数使用 `apiClient`（Axios），自动注入 `Bearer` token
- **数据缓存**：TanStack Query 管理请求缓存、加载状态、自动重试
- **全局状态**：`auth-store` 管理用户身份、Token、权限信息
- **响应解析**：`response.data.data` 获取业务数据（外层 `.data` 是 Axios，内层 `.data` 是 `domain.Response.Data`）

## 认证流程

```mermaid
sequenceDiagram
    participant U as 用户
    participant FE as 前端
    participant BE as 后端
    participant DB as 数据库

    U->>FE: 输入用户名/密码
    FE->>BE: POST /api/v1/auth/login
    BE->>DB: 校验凭据 (bcrypt)
    BE->>BE: 生成 Access Token (短期) + Refresh Token (长期)
    BE-->>FE: { accessToken, refreshToken }
    FE->>FE: 存储 Token (localStorage + Cookie)
    FE->>FE: 解析 JWT 获取用户基本信息
    FE->>BE: GET /api/v1/profile (带 Bearer Token)
    BE-->>FE: 用户详细信息
    FE->>BE: GET /api/v1/resources (带 Bearer Token)
    BE-->>FE: 菜单 + 权限列表
    FE->>FE: 渲染侧边栏 + 启用权限控制
```

## 权限模型

Shadmin 使用 **Casbin RBAC** 进行权限控制：

```
用户 → 角色 → 权限策略 → API 资源（path + method）
              ↓
          菜单绑定 → 前端侧边栏 + 按钮显隐
```

**后端**：Casbin 中间件检查 `(userID, requestPath, requestMethod)` 是否匹配策略。  
**前端**：`auth-store` 提供 `hasPermission()` / `hasRole()` / `canAccessMenu()` 方法，`PermissionButton` / `PermissionGuard` 组件控制 UI 显隐。

### 菜单与资源机制

1. 后端启动时，`bootstrap.InitApiResources()` 扫描所有 Gin 路由，以 `METHOD:/path` 为 ID 写入数据库
2. 管理员在菜单管理中将 API 资源绑定到菜单项
3. 角色分配菜单后，Casbin 策略自动同步
4. 前端通过 `/api/v1/resources` 获取当前用户可见的菜单树和权限列表，动态渲染侧边栏

## 数据库抽象

通过 `.env` 中的 `DB_TYPE` 切换数据库，代码无需任何修改：

| DB_TYPE | 说明 | DB_DSN 示例 |
|---------|------|------------|
| `sqlite` | 默认，零配置 | 留空（自动使用 `.database/data.db`） |
| `postgres` | 生产推荐 | `postgres://user:pass@localhost:5432/shadmin?sslmode=disable` |
| `mysql` | 可选 | `user:pass@tcp(localhost:3306)/shadmin?parseTime=true&loc=Local` |

启动时 Ent ORM 自动执行 schema 迁移。

## 文件存储抽象

通过 `STORAGE_TYPE` 切换存储后端：

| STORAGE_TYPE | 说明 | 关键配置 |
|-------------|------|---------|
| `disk` | 默认，本地磁盘 | `STORAGE_BASE_PATH=./uploads` |
| `minio` | MinIO / S3 兼容存储 | `S3_ADDRESS`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` |

实现统一的 `domain.FileRepository` 接口，业务层无感切换。

## 启动流程

```
main.go
  → cmd.Run()
    → bootstrap.App()              # 加载 .env、连接 DB、初始化 Casbin/存储/Gin
    → api.SetupRoutes(app)         # 注册静态资源、Swagger、API 路由
    → bootstrap.InitApiResources() # 扫描 Gin 路由写入 DB
    → bootstrap.InitDefaultAdmin() # 初始化 admin 角色/用户、绑定菜单与策略
    → bootstrap.InitDictData()     # 初始化字典数据
    → bootstrap.InitCasbinHooks()  # 设置 Casbin 规则
    → api.Run(app)                 # 监听端口 :55667
```

## 前后端协作

- **生产模式**：前端构建产物 (`web/dist/`) 通过 `web/web.go` 以 Go embed 方式嵌入二进制，由后端统一提供 SPA + API 服务
- **开发模式**：前端 Vite 开发服务器运行在 `:5173`，通过 `vite.config.ts` 中的 proxy 将 `/api` 请求代理到后端 `:55667`

## 下一步

- [开发指南](./development.zh.md) — 基于脚手架新增功能模块
- [部署指南](./deployment.zh.md) — 生产环境部署
