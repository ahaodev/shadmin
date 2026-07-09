# Redis 支持实施计划

> 本文为可执行实施计划，待评审通过后开工。方案见 `docs/redis-support.md`。
> 约定：`REDIS_ADDR` 留空 = 全部走内存默认；填写即 casbin/jwt/captcha 统一切 Redis。

> **状态（实施记录）**
> - 阶段 0-3 已实现并通过 `go fmt / vet / build / test`（无测试包，全量绿）。
> - 实际实现与初版有三处取舍：
>   - Casbin 适配器使用 `casbin-redis-adapter`（`NewAdapter`/`NewAdapterWithPassword`，基于 redigo；未使用 `NewAdapterWithClient`，go-redis client 不直接复用）。
>   - JWT 黑名单接口 `Add(ctx, jti, expiry time.Time)`（而非 `ttl Duration`），过期由底层 TTL 处理。
>   - userstatus `Store` 接口仅 `Get/Set/Invalidate`；`InvalidateAll` 保留在 `Cache` 上，仅内存实现生效，Redis 依赖 key TTL。
> - 阶段 4：`docs/getting-started/deployment.{zh,en}.md` 已补 Redis 配置段；`.env.example` 在阶段 0 已补。

## 总体

- 依赖：`github.com/redis/go-redis/v9`、`github.com/casbin/data-adapter/v2`、`github.com/casbin/redis-adapter/v2`
- 装配：`bootstrap/app.go` 按 `REDIS_ADDR` 是否非空构造 `*redis.Client`（空则 `nil`），注入各模块；`nil` 客户端 → 各模块走内存实现
- 分层：`domain` 定义接口、`internal/` 提供双实现、`bootstrap` 装配
- 验收：每阶段 `go fmt ./... && go vet ./... && go test ./...`；不改前端

---

## 阶段 0：基础设施与配置（0.5d）

**改动文件**
- `go.mod` / `go.sum`：`go get github.com/redis/go-redis/v9 github.com/casbin/data-adapter/v2 github.com/casbin/redis-adapter/v2`
- `bootstrap/env.go`
  - `Env` 新增字段：`RedisAddr`(`REDIS_ADDR`)、`RedisPassword`(`REDIS_PASSWORD`)、`RedisDB`(`REDIS_DB`) （int，默认 0）
  - `setDefaults()`：三项默认值（`REDIS_ADDR=""`、`REDIS_PASSWORD=""`、`REDIS_DB=0`）
  - `generateEnvFile()`：新增「# Redis 配置」段
  - `validate()`：`REDIS_ADDR` 非空时校验格式；`REDIS_DB` ∈ [0,15]
- `.env.example`：同步新增 Redis 配置段与注释

**新增文件**
- `internal/cache/client.go`
  - `type Client = *redis.Client`（便于 nil 表示未启用）
  - `func NewClient(env *Env) *redis.Client`：`RedisAddr==""` 返回 `nil`；否则 `redis.NewClient(...)` 并 `PING` 自检
  - `func Health(ctx, c) error`：nil → nil；否则 `c.Ping(ctx)`
- `internal/cache/client_test.go`：nil 路径返回 nil；未启用不发起新连接

**装配**
- `bootstrap/app.go: App()`
  - `app.Redis = cache.NewClient(app.Env)` （新增 `Redis *redis.Client` 字段）
  - `CloseDBConnection()`：`if app.Redis != nil { _ = app.Redis.Close() }`

**验收**：默认 `REDIS_ADDR=""` 启动如常；填写后 PING 通过。

---

## 阶段 1：Casbin 适配器切换（0.5d，P1）

**原则**：`SyncService` + Ent Hook + 1h 兜底不动；仅 Enforcer 适配器随 `REDIS_ADDR` 切换。

**改动文件**
- `internal/casbin/manager.go`
  - `initializeCasbin(entClient, redisClient *redis.Client)`（新增参数）
    - `redisClient == nil`（或 `REDIS_ADDR` 空）：`enforcer, err = casbin.NewEnforcer(m)`（无适配器）；**不调 `LoadPolicy`**（无来源）
    - `redisClient != nil`：`adapter, _ := redisadapter.NewAdapterWithClient(redisClient)`；`enforcer, _ = casbin.NewEnforcer(m, adapter)`；`EnableAutoSave(true)`；`LoadPolicy()`（Redis 空则加载空，首启由 `InitCasbinHooks` 全量重灌）
  - `SavePolicy()`：`if m.enforcer == nil || m.adapter == nil { return nil }`；否则 `return m.enforcer.SavePolicy()`
    - 说明：`sync.go` 的 `SyncFromDatabase`(L61)、`SyncUserRole`(L215)、`SyncRolePermissions`(L264) 显式调 `SavePolicy()`，遇 nil adapter 会报 `InvalidDataAdapter`；AutoSave 路径遇 nil 是静默 no-op，无需处理
  - `NewCasManagerWithLogger` / `NewCasManager` / `Initialize`：签名传入 `redisClient`，存入 `CasManager.adapter` 字段用于 `SavePolicy` 判断
- `bootstrap/app.go`
  - `casbin.Initialize(app.DB, app.Redis)`
- `bootstrap/casbin_init.go`：`NewCasbinInitializer` 无需改（依赖 Manager 接口）

**新增测试**
- `internal/casbin/manager_test.go`
  - memory：`SavePolicy()` 不报错；`AddPolicy` 仅落内存、重启（重建 Enforcer）消失
  - redis（可选，需 miniredis 或跳过）：`AddPolicy` 后落 Redis；重建 Enforcer `LoadPolicy` 恢复

**legacy 处理**：自研 Ent 适配器（`internal/casbin/adapter.go`）与 `ent/schema/casbinrule.go` 阶段内**不删**，仅在 `redisClient != nil` 时不装配；待稳定后另起 PR 评估移除。

**验收**：`REDIS_ADDR=""` 启动权限检查正常；填写后重启策略仍在（Redis 持久化）。

---

## 阶段 2：Captcha Store（1d，P1）

**改动文件**
- `internal/captcha/slide_manager.go`
  - 抽出 `ChallengeStore` 接口：
    ```go
    type ChallengeStore interface {
        Save(ctx, id string, rec challengeRecord, ttl time.Duration) error
        Get(ctx, id string) (challengeRecord, bool, error)
        MarkUsed(ctx, id string) error
        Delete(ctx, id string) error
        IncrAttempts(ctx, id string) (int, error)
    }
    ```
  - `challengeRecord` 增 `omitempty` JSON tag 便于序列化
  - `SlideManager`：`mu sync.Mutex` + `challenges map` → 替换为 `store ChallengeStore`；`Generate/Verify/Invalidate` 改走 store；`stopCh` 清理 goroutine 仅 memory store 需要
- **新增** `internal/captcha/memory_store.go`：基于现有 map + mutex 实现 `ChallengeStore`，含后台过期清理（保留 `startCleanup`）
- **新增** `internal/captcha/redis_store.go`：
  - Key `captcha:challenge:{id}`，Hash 存 x/y/used/attempts（或 JSON string），TTL 15s
  - `Get` 用 `HGetAll`；未命中返回 `(zero, false, nil)`；自动按 TTL 过期，无后台清理
  - `IncrAttempts` 用 `HIncr`
- `internal/captcha/slide_manager.go: NewSlideManager`
  - 签名改为 `NewSlideManager(store ChallengeStore)`；构造 captcha 不变
- `api/route/factory.go: NewControllerFactory`
  - `store := captchapkg.NewMemoryStore()`；若 `app.Redis != nil` 改 `captchapkg.NewRedisStore(app.Redis)`
  - `cm, err := captchapkg.NewSlideManager(store)`
- `bootstrap/app.go: CloseDBConnection()`：若 SlideManager 实现 `Close`，调用以停清理 goroutine（factory 持有，需把 manager 移到 app 或 factory 提供 Close）

**新增测试**
- `internal/captcha/memory_store_test.go` / `redis_store_test.go`
  - Save→Get 一致；TTL 过期 Get 返回 false；MarkUsed 后 Get used=true；IncrAttempts 递增到 maxAttempt 删除
  - redis 测试用 miniredis 或 `SHORT=1` 跳过

**验收**：生成/校验/失效语义不变；`REDIS_ADDR` 填写后多实例共享（手动跨实例校验）。

---

## 阶段 3：JWT 黑名单 + userstatus（2d，P0）

### 3.1 JTI 落地

**改动文件**
- `domain/jwt_custom.go`
  - `JwtCustomClaims` 增 `RegisteredClaims.ID` 即 JTI（用 `jwt.RegisteredClaims.ID`，无需新字段）；`JwtCustomRefreshClaims` 同理
- `internal/tokenutil/tokenutil.go`
  - `CreateAccessToken`/`CreateRefreshToken`：`jti := xid.New().String()`；`RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: exp, ID: jti}`
  - 返回值一并携带 jti（供 controller 写黑名单）：新增 `CreateAccessTokenWithJTI` 或返回 `(token, jti, err)`；考虑向前兼容，新增方法、旧方法内部调用并丢 jti

### 3.2 TokenBlacklist 接口

**新增文件**
- `domain/token_blacklist.go`
  ```go
  type TokenBlacklist interface {
      Add(ctx, jti string, ttl time.Duration) error
      Exists(ctx, jti string) (bool, error)
  }
  ```
- `internal/auth/tokenblacklist/memory.go`：`go-cache` 实现，TTL 自动过期
- `internal/auth/tokenblacklist/redis.go`：Key `jwt:blacklist:{jti}`，`SET ... EX`；`Exists` 用 `EXISTS`
- `internal/auth/tokenblacklist/*_test.go`：Add→Exists true；过期 Exists false

### 3.3 userstatus 接口化

**改动文件**
- `internal/userstatus/cache.go`
  - 抽出接口（或新增）：`type StatusCache interface { Get(ctx, userID) (string, error); Invalidate(userID); InvalidateAll() }`
  - `Cache` 改名/保留为实现 `MemoryCache`；新增 `internal/userstatus/redis_cache.go`
    - Redis `user:status:{id}` TTL 30s；miss 回源 `Loader` 后 `SET ... EX 30`
    - `Invalidate` → `DEL`；`InvalidateAll` → 删除困难（SCAN `user:status:*`）或记录版本号；采用简化：InvalidateAll 仅测试用，redis 实现返回 nil 或 FLUSHDB 不可行 → 改为 no-op + log
- `bootstrap/app.go`
  - `app.UserStatusCache`：`app.Redis != nil` 用 `redis_cache`，否则 `userstatus.New(...)`
- `registerUserStatusCacheHook`：`Invalidate` 走接口即可，不变

### 3.4 Logout 与中间件接线

**改动文件**
- `api/controller/auth_controller.go`
  - `AuthController` 新增字段 `TokenBlacklist domain.TokenBlacklist`
  - `Logout`：从 access/refresh token 提取 jti 与 `ExpiresAt`；`TokenBlacklist.Add(jti, time.Until(exp))`（null blacklist 则跳过）
- `api/middleware/jwt_auth_middleware.go`
  - 签名校验通过、userstatus 检查之前/之后插入黑名单检查：`if blacklist != nil { if ok, _ := blacklist.Exists(ctx, claims.ID /* jti */); ok { 401 } }`
  - `JwtAuthMiddleware(secret string, userStatusCache, blacklist domain.TokenBlacklist)`
- `api/route/factory.go`
  - `CreateAuthController` 注入 `TokenBlacklist`（app 装配后传入）
  - 中间件挂载处同步传 `blacklist`

**验收**
- memory：登出后同 jti 请求 401；过期后黑名单消失
- redis：跨实例登出生效
- `TokenBlacklist == nil` 时行为等价现状（向后兼容）

---

## 阶段 4：文档与验收（0.5d）

- `docs/getting-started/deployment.*.md`：新增 Redis 部署段
- `docs/getting-started/development.*.md`：本地开发 `REDIS_ADDR` 留空说明
- `.env.example` 也许补 Redis 段（阶段 0 已做）
- 全量回归：`go fmt ./... && go vet ./... && go test ./...`；Swagger 注解变更若涉及则 `go generate ./...`
- 手工冒烟：默认内存启动正常；配 Redis 后权限/验证码/登录登出正常

---

## 风险与回滚

| 风险 | 缓解 |
|------|------|
| Redis 断连 | 各集成点 `app.Redis == nil` 降级内存；运行中断连由 go-redis 重试，黑名单短时可能漏判（可接受，等价现状） |
| `sync.go` SavePolicy 在 memory 模式报错 | `SavePolicy()` nil 适配器短路（阶段 1） |
| JTI 引入导致旧 token 无效 | 旧 token 无 JTI，`Exists` 查空 key → false，不会误拒；黑名单对旧 token 不生效（可接受，渐进失效） |
| userstatus `InvalidateAll` 在 redis 难实现 | 仅测试用，redis 实现 no-op |
| legacy casbinrule 表残留 | 不删，避免 schema 变更；另起 PR 评估 |

回滚：各阶段独立提交，按阶段 revert 即可；无 schema 强依赖（casbinrule 保留）。

---

## 提交建议

- `feat(cache): add redis client and env config`（阶段 0）
- `feat(casbin): switch enforcer adapter by redis config`（阶段 1）
- `feat(captcha): abstract challenge store with memory/redis`（阶段 2）
- `feat(auth): jwt blacklist via jti and redis`（阶段 3）
- `docs: redis deployment notes`（阶段 4）
