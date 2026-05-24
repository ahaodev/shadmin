# shadmin-cli

基于 [Shadmin](../README.md) REST API 构建的轻量级 Go CLI，专为外部 **AI 智能体** 集成设计。

- 独立 Go 模块（`cli/`），构建产物为单一二进制文件 `shadmin-cli`。
- 默认输出 **JSON**（稳定，适合智能体解析）。`--pretty` 切换为人类可读的表格格式。
- 认证：OAuth 设备授权流程 → JWT 缓存至 `cli/.env` 或 `SHADMIN_CONFIG` 指定路径（文件权限 `0600`）。
- 鉴权：每次调用均沿用已登录用户的 RBAC 权限，CLI **无法绕过**服务端权限检查。
- MVP 版本为**只读**，不开放 `create / update / delete` 命令。

## 安装 / 构建

```bash
cd cli
make build           # → shadmin-cli
# 或者：
make install         # 安装到 $GOBIN
```

二进制文件通过 `-ldflags` 注入版本元数据：
`shadmin-cli --version` 输出 `version (commit X, built Y)`。

## 配置

CLI 配置作用域限定在 `cli/` 目录下，请勿将 CLI 专属配置添加到仓库根目录的 `.env` / `.env.example`（这些文件仅供后端服务使用）。

仓库本地示例：

- `cli/.env.example` — 服务地址配置（`SHADMIN_SERVER`）

`cli/.env` 由 `shadmin-cli login` 自动生成和维护，作为本地 token 缓存。也可将 `cli/.env.example` 复制为 `cli/.env` 以配置本地服务地址。

| 来源                        | 优先级   |
|-----------------------------|----------|
| `--server` 标志             | 最高     |
| `SHADMIN_SERVER` 环境变量   |          |
| `cli/.env`                  | 最低     |

使用 `SHADMIN_CONFIG=/some/path.env` 可完全覆盖配置文件路径。

## 命令（MVP）

```text
shadmin-cli
├── login            [--server URL]
├── logout
├── whoami
├── users
│   ├── list         [--page N] [--page-size N] [--keyword K]
│   └── get <id>
├── roles
│   ├── list
│   └── get <id>
├── menus
│   ├── tree
│   ├── list
│   └── get <id>
└── api-resources list
```

全局标志：`--pretty`、`--server URL`。默认输出格式为 JSON。

## 快速开始

```bash
# 1. 使用设备授权登录
shadmin-cli login --server http://localhost:55667
# 在浏览器中打开输出的 URL，并输入显示的用户码。

# 2. 验证登录状态
shadmin-cli whoami

# 3. 查询数据
shadmin-cli users list --page 1 --page-size 10
shadmin-cli menus tree --pretty
shadmin-cli api-resources list
```

## 退出码

| 退出码 | 含义                                         |
|--------|----------------------------------------------|
| `0`    | 成功                                         |
| `1`    | 通用错误                                     |
| `2`    | 用法错误（缺少参数或标志）                   |
| `3`    | 网络错误                                     |
| `4`    | 未认证或 token 已过期                        |
| `5`    | 权限不足（HTTP 403）                         |
| `6`    | 资源不存在（HTTP 404）                       |
| `7`    | 服务端错误（HTTP 5xx 或无效响应）            |

401 路径由 CLI 自动处理：在返回退出码 `4` 前，会自动尝试刷新 access token 一次。

## 安全说明

- CLI 继承已登录用户的**全部** RBAC 权限，请将缓存的配置文件视为敏感凭据妥善保管。
- token 文件以 `0600` 权限写入，请勿放宽该权限。
- MVP 版本故意不实现写操作。如需写操作，请先在后端提出方案，确保变更经过 Casbin 审查。

## AI 智能体集成

在 [`skill/shadmin-cli/SKILL.md`](skill/shadmin-cli/SKILL.md) 提供了开箱即用的 Anthropic Skill，
JSON 输出示例位于 [`skill/shadmin-cli/examples/`](skill/shadmin-cli/examples/)。

## 目录结构

```
cli/
├── main.go
├── cmd/                # cobra 命令（root、auth、resources）
├── internal/
│   ├── client/         # HTTP 客户端、响应体解析、401 自动刷新
│   ├── config/         # cli/.env 配置文件及环境变量覆盖
│   ├── output/         # JSON / 表格渲染
│   └── clierr/         # 退出码及错误包装
├── skill/shadmin-cli/  # Anthropic SKILL.md 及 AI 智能体示例
├── Makefile
└── README.md
```

## 测试

```bash
make test   # config + client 单元测试（基于 httptest）
```

## 路线图

以下功能明确**不在 MVP 范围内**，将在后续迭代中跟进：

- 写命令（create / update / delete），需配合更严格的审计机制。
- 后端审计字段，用于区分 CLI 与 Web 来源。
- 在同一客户端层之上可选的 MCP Server 封装。
