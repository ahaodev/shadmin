# Redis 支持方案

> 状态：待实施　更新：2026-07-09

## 现状

| 模块 | 现状 | 问题 |
|------|------|------|
| Casbin | 自研 Ent 适配器，策略落 `casbinrule` 表 | 与业务表同库冗余；多实例靠 Hook + 1h 兜底 |
| Captcha | 进程内 `map` + 后台清理 | 多实例不共享、重启即丢 |
| JWT | 纯无状态，无黑名单 | 登出后 Token 仍可用至过期 |
| userstatus | `patrickmn/go-cache` 30s TTL，内存 | 跨实例禁用延迟 |

真相源：应用表（User/Role/Menu，PostgreSQL）；`SyncService` 经 Manager 接口单向同步进 Enforcer，AutoSave 落当前适配器。

## 方案

Redis 为可选基础设施，默认关闭。各集成点「接口 + 双实现」，`memory` 默认兜底，`redis` 可选增强。

### 1. Casbin 适配器

`SyncService` + Ent Hook + 1h 兜底不动，仅替换 Enforcer 适配器。随 `REDIS_ADDR` 是否填写切换：

| `REDIS_ADDR` | 构造 | 持久化 |
|------|------|--------|
| 留空（默认） | `NewEnforcer(model)`，无适配器 | 无，重启重灌 |
| 填写 | `NewEnforcer(model, redisadapter)` + AutoSave | Redis |

改动两处：
1. `initializeCasbin()`：按 `REDIS_ADDR` 是否非空选择是否注入 `casbin/redis-adapter/v2`
2. `SavePolicy()`：无适配器时短路返回 `nil`（`sync.go` 显式调用遇 nil adapter 报 `InvalidDataAdapter`；AutoSave 路径静默 no-op）

legacy 自研 Ent 适配器与 `casbinrule` schema 先保留不装配，稳定后再评估删除。

### 2. Captcha Store

抽象 `ChallengeStore` 接口（`Save/Get/MarkUsed/IncrAttempts`）：
- `MemoryStore`（现有 `map`）/ `RedisStore`（Hash + 15s TTL）
- Redis 模式可移除后台清理 goroutine

```
KEY: captcha:challenge:{id}  TTL: 15s
```

### 3. JWT 黑名单 + userstatus

`TokenBlacklist` 接口（`Add(token, ttl) / Exists`），memory/redis 双实现：
- `Logout` 写入 access+refresh（TTL = 剩余有效期）
- `jwt_auth_middleware` 签名校验后查黑名单
- claims 补 `JTI` 作 key
- `userstatus.Cache` 同步接口化

```
KEY: jwt:blacklist:{jti}    TTL: 剩余有效期
KEY: user:status:{userID}   TTL: 30s
```

## 落地

### 配置

```dotenv
REDIS_ADDR=                 # 留空 = 全部走内存默认；填写即启用，casbin/jwt/captcha 统一切 Redis
REDIS_PASSWORD=
REDIS_DB=0
```

### 依赖

- `github.com/redis/go-redis/v9`
- `github.com/casbin/data-adapter/v2`
- `github.com/casbin/redis-adapter/v2`（apache/casbin-redis-adapter 重定向至此）

### 阶段

| 阶段 | 内容 | 优先级 | 工作量 |
|------|------|--------|--------|
| 0 | go.mod 依赖；`env.go` + `.env.example` 配置；`internal/cache` 客户端工厂；`app.go` 装配 | — | 0.5d |
| 1 | Casbin 适配器（`initializeCasbin` 随 `REDIS_ADDR` 切换 + `SavePolicy` 短路 + 单测） | P1 | 0.5d |
| 2 | Captcha Store 接口化 + 双实现（单测） | P1 | 1d |
| 3 | JWT 黑名单（JTI + Logout + 中间件）+ userstatus 接口化（单测） | P0 | 2d |
| 4 | 更新 deployment/development 文档；全量回归 | — | — |

### 关键决策

- Casbin 真相源始终是应用表，Redis 仅替适配器落库位置，不做额外一致性处理（多实例等价现状）
- `REDIS_ADDR` 留空 = 零 Redis 依赖，最小部署形态不变
- 所有集成点 memory/redis 双实现，Redis 不可用自动降级
- JWT 黑名单以 `JTI` 为 key，需在 claims 补字段
