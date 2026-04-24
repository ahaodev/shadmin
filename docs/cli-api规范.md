# Shadmin CLI API 规范

> **最后更新**: 2026-04-24 | **用途**: CLI 安全边界、功能设计、认证规范

---

## 核心价值

| 场景 | 用途 |
|------|------|
| **AI Agent** | 自然语言 → CLI 命令 → JSON → 智能决策 |
| **DevOps** | 脚本化审计、CI/CD 验证、自动化备份 |
| **跨平台** | SSH/容器/Ansible 均可使用，Web UI 做不到 |

**核心设计**：`CLI 权限 = 登录用户权限`，通过 JWT 继承，所有操作经后端审计。

---

## 命令白名单（已实现 ✅ / 规划中 🔜）

### 认证
```bash
✅ shadmin-cli login                          # 用户名密码登录，缓存 JWT
✅ shadmin-cli login --password-stdin         # CI/CD 非交互登录
✅ shadmin-cli logout                         # 清除本地 token
✅ shadmin-cli whoami                         # 当前用户信息
🔜 shadmin-cli whoami --show-permissions      # + 权限清单
🔜 shadmin-cli login --device                 # Device Flow（见下文）
```

### 查询资源（只读）
```bash
✅ shadmin-cli users list [--page N] [--page-size N] [--keyword X]
✅ shadmin-cli users get <id>
🔜 shadmin-cli users get <id> --show-permissions

✅ shadmin-cli roles list
✅ shadmin-cli roles get <id>
🔜 shadmin-cli roles get <id> --show-permissions

✅ shadmin-cli menus tree
✅ shadmin-cli menus list
✅ shadmin-cli menus get <id>

✅ shadmin-cli api-resources list

🔜 shadmin-cli login-logs list [--user-id X] [--days N]  # 只读，不可删
🔜 shadmin-cli auth check-permission <method> <path>
```

### 个人自服务（v2）
```bash
🔜 shadmin-cli profile get                    # whoami 别名
🔜 shadmin-cli profile update-password        # 交互式，需要原密码
```

---

## 操作黑名单（永不暴露 🚫）

| 操作 | 原因 |
|------|------|
| `users / roles / menus` **create / update / delete** | 只能通过 Web UI，防止 CLI 滥用 |
| `login-logs delete` | 审计日志不可抹除，法律合规要求 |
| 修改**他人**信息或密码 | 身份冒充风险 |
| 直接数据库访问、master key | 绕过审计链 |
| 输出数据库 DSN、JWT 密钥、S3 凭证 | 敏感凭证泄露 |

---

## 认证方案

### 当前：用户名 + 密码

```bash
shadmin-cli login -u admin -p xxx  # 或 --password-stdin
# 缓存至 ~/.shadmin/config.yaml（access_token + refresh_token）
```

### Device Authorization Flow（RFC 8628）🔜

适用于无密码场景：AI Agent、第三方集成、OAuth 接入。

**流程**：
```
CLI ──POST /api/v1/auth/device/code──► 后端
     ◄── device_code + user_code ─────
     
用户看到：
  📋 授权码: WDJB-MJHT
  🔗 访问: http://your-shadmin/device

CLI ──轮询 /api/v1/auth/device/token──► 后端（每5s）
     ◄── authorization_pending ─────── （用户未确认）
     ◄── access_token ──────────────── （用户已授权 ✅）
```

**后端需新增两个端点**：

```
POST /api/v1/auth/device/code
  Request:  { client_id: "shadmin-cli" }
  Response: { device_code, user_code, expires_in: 900, interval: 5 }

POST /api/v1/auth/device/token
  Request:  { client_id, device_code }
  Response: { access_token, refresh_token } | { error: "authorization_pending" }
```

**前端需新增授权页** `/device`：输入 `user_code` → 确认授权。

**CLI 实现要点**（`cli/cmd/auth.go`）：
```go
// login --device 流程
resp, _ := client.RequestDeviceCode()
fmt.Printf("📋 授权码: %s\n🔗 请访问: %s\n", resp.UserCode, resp.VerificationURI)

ticker := time.NewTicker(time.Duration(resp.Interval) * time.Second)
for range ticker.C {
    token, err := client.PollDeviceToken(resp.DeviceCode)
    if err == nil { saveToken(token); break }  // 成功
    if err.Error() == "expired_token" { return errExpired }  // 超时
    // "authorization_pending" → 继续等待
}
```

**安全要点**：
- `user_code` 有效期 15 分钟，过期重新申请
- `device_code` 对用户不可见，防止暴力破解
- 轮询过快返回 `slow_down` 错误
- 用户在 Shadmin 前端明确确认授权范围

---

## 安全设计原则

| 原则 | 规则 |
|------|------|
| **最小权限** | CLI 权限 ≤ 登录用户权限，无超级 token |
| **审计不可绕过** | CLI → REST API → Casbin → 业务 → 日志，不可短路 |
| **敏感信息不输出** | 错误信息不含 DB host/JWT key/凭证，统一模糊处理 |
| **写操作走 Web** | CLI 定位为只读 + 自服务，破坏性写操作必须经 UI 确认 |

---

## 添加新命令检查清单

```
□ 是否只读？写操作 → 是否有充分审计？
□ 非 admin 调用是否安全？→ 后端验权
□ 是否涉及密钥/密码？→ 不返回
□ 是否可删除关键数据？→ 不暴露
□ 错误信息是否泄露内部信息？→ 模糊处理
□ 是否有集成测试？→ 先测试再上线
```

---

## 路线图

| 阶段 | 功能 | 状态 |
|------|------|------|
| **MVP** | users/roles/menus/api-resources 查询、login/logout/whoami | ✅ |
| **v2** | `--show-permissions`、login-logs 只读、check-permission | 🔜 |
| **v3** | profile update-password（交互式）、数据导出 CSV | 🔮 |
| **v4** | Device Authorization Flow、MCP 服务器模式 | 🔭 |

---

## 相关文档

- [CLI README](../cli/README.md) · [CLI Skill](../cli/skill/shadmin-cli/SKILL.md)
- [API 路由](../api/route/) · [Casbin 权限](../internal/casbin/)

---

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-04-24 | 1.0 | 初版：安全边界、功能矩阵、原则 |
| 2026-04-24 | 1.1 | 精简结构，新增 Device Flow 认证规范 |
