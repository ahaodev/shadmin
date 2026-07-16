# 第三方登录身份归并方案（provider-aware）

## 1. 背景

当前社交登录逻辑已经具备“按第三方 provider + subject 查绑定，再按邮箱查用户”的基础能力。

但在实际使用中，邮箱相同并不等于“就是同一个人”。例如：

- Google 登录时返回 `alice@example.com`
- GitHub 登录时也返回 `alice@example.com`

这类情况需要增加“provider 维度判断”，避免误合并。

## 2. 目标

实现一个“provider-aware”的身份归并策略，满足以下目标：

- 同 provider + 同 subject：必定归并到同一个本地用户。
- 同 email + 允许的 provider 组合：可以自动归并。
- 同 email + 不允许的 provider 组合：不自动归并，避免误判。
- 头像只用于展示，不参与身份归并。
- 现有 `social_accounts` 结构尽量不做大改，优先在业务逻辑层实现。

## 3. 归并规则

建议采用“默认拒绝，显式允许”的策略。

### 3.1 规则优先级

1. 先按 `(provider, provider_subject)` 查已绑定账号
2. 如果没有命中，再判断邮箱是否可用于归并
3. 只有在满足 provider 规则时，才允许按邮箱归并
4. 其他情况走“新建用户”或“待人工绑定”流程

### 3.2 规则表

| 场景 | 处理方式 |
| --- | --- |
| 同 provider + 同 subject | 直接绑定到同一个本地用户 |
| 同 provider + 同 email | 允许自动归并 |
| 不同 provider + 同 email | 仅在 provider 允许组合时自动归并 |
| 不同 provider + 同 email，但 provider 未允许 | 不自动归并，创建新用户或进入待绑定状态 |
| 邮箱为空 / 占位邮箱 / 不可信邮箱 | 不自动归并 |

### 3.3 推荐 provider 组合

当前建议先只对以下组合做自动归并：

- `google` ↔ `github`

对其他 provider（如 `wechat`、`apple`、`microsoft` 等）默认不做跨 provider 自动归并，避免误判。

后续如果业务确认需要扩展，可以通过配置或白名单扩容。

## 4. 具体决策逻辑

### 4.1 归并判定函数

新增一个归并策略函数，例如：

```go
func canAutoMerge(provider, email string, existingUser *domain.User) bool
```

其内部逻辑建议为：

- 邮箱不能为空
- 邮箱不是占位邮箱（如 `google_xxx@social.local`、`github_xxx@social.local`）
- 当前 provider 是否在允许配对列表中
- 目标用户是否已经存在，且没有冲突

### 4.2 归并顺序

在 `socialLoginUsecase.HandleCallback()` 中，建议调整为：

```text
1. 先查 provider+subject 的绑定
2. 若未命中，查看是否可以按邮箱归并
3. 若允许归并，绑定到已有用户
4. 若不允许归并，创建新用户
5. 绑定当前 provider 的社交账号
```

### 4.3 冲突处理

如果出现下面情况，建议不要自动合并：

- 同一个邮箱命中了多个不同的本地用户
- 当前 provider 与现有绑定的 provider 组合不在允许列表中
- 现有用户已经存在“不同 provider 的高可信绑定”，但当前登录的身份不明确

这种情况下可以选择：

- 创建新用户（保守策略）
- 或者记录为“待人工确认”状态（更稳妥）

建议先采用“创建新用户 + 记录日志”的保守策略，避免误合并。

## 5. 数据模型建议

### 5.1 现有表可继续使用

当前 `social_accounts` 表已经足够支撑：

- `provider`
- `provider_subject`
- `user_id`
- `email`
- `name`
- `avatar_url`

因此不建议一开始就引入复杂的“身份合并主表”。

### 5.2 可选增强字段（后续）

如果后续需要更严谨的审计能力，可以考虑增加：

- `identity_verified_at`
- `is_primary`
- `merge_source`

但这不是第一阶段必须项。

## 6. 头像处理策略

头像不参与身份归并。建议按以下优先级处理：

1. 用户手动设置头像优先
2. 之前已绑定的头像优先
3. 当前 provider 的头像仅作为兜底更新

也就是说：

- 头像决定“展示内容”
- provider/email 决定“身份归并”

## 7. 实现步骤

### 第一步：新增 provider 归并策略

在 `usecase` 层新增一个策略对象或工具函数，负责判断：

- 当前 provider 是否允许与目标用户绑定
- 当前邮箱是否可信
- 是否需要自动归并

### 第二步：改写社交登录用户解析流程

把当前逻辑从“邮箱匹配即可归并”改成：

- 先按 provider+subject 查
- 再判断 provider 规则
- 再按邮箱归并
- 最后再创建新用户

### 第三步：补测试

至少覆盖以下场景：

- Google → GitHub 同邮箱，允许归并
- Google → GitHub 同邮箱，但邮箱为空，不归并
- Google → GitHub 同邮箱，但 provider 不在白名单，不归并
- 同 provider + 同 subject，必须归并
- 同 email 命中多个用户，必须不自动归并

### 第四步：上线前观察

上线后重点观察：

- 自动归并成功率
- 误合并次数
- 新用户创建次数
- 日志里是否出现冲突场景

## 8. 推荐落地方式

为了降低风险，建议按“保守自动合并”方式落地：

- 只允许 `google` 和 `github` 的跨 provider 自动归并
- 其他 provider 默认不自动归并
- 只对非占位邮箱自动归并
- 出现冲突时不自动合并

这样做的好处是：

- 兼容当前业务
- 风险低
- 后续可以逐步放开更多 provider

## 9. 结论

如果要处理“同一个邮箱，但还要判断 provider”的场景，最稳妥的方案不是“仅看邮箱”，而是：

- 先看 provider+subject
- 再看 provider 组合是否允许
- 再看邮箱是否可信
- 最后才决定是否归并

这能在保证用户体验的同时，显著降低误合并的风险。
