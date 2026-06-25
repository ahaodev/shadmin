# 部门管理功能评估

> 评估日期：2026-04-10
> 状态：待实施

## 1. 功能概述

在现有 Shadmin RBAC 系统中新增**部门管理（Department）**模块，实现组织架构的树形管理。用户归属于某一个部门，权限体系保持基于角色不变。

### 1.1 需求确认

| 决策项 | 结论 |
|--------|------|
| 部门层级 | 支持多层嵌套树形结构（无限级） |
| 用户与部门关系 | 一对多：一个用户只属于一个部门 |
| 删除策略 | 删除部门前需先移除所有子部门和员工 |
| 权限集成 | 不与权限系统耦合，权限继续基于角色 |

### 1.2 部门字段定义

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | string (xid) | PK, 20字符, 不可变 | 部门唯一标识 |
| parent_id | *string | FK → department.id, 可空 | 上级部门 ID，顶级部门为 nil |
| name | string | 必填, 最大100字符 | 部门名称 |
| sequence | int | 默认 0 | 排序序号（同级排序） |
| leader | string | 可选, 最大64字符 | 负责人姓名 |
| phone | string | 可选, 最大20字符 | 联系电话 |
| email | string | 可选, 最大100字符 | 邮箱 |
| status | enum | active / inactive, 默认 active | 部门状态 |
| created_at | time | 不可变, 自动设置 | 创建时间 |
| updated_at | time | 自动更新 | 更新时间 |

---

## 2. 技术方案

### 2.1 整体架构（遵循现有 Clean Architecture）

```
Route → Middleware (JWT + Casbin) → Controller → Usecase → Repository → Ent/DB
```

新增文件清单：

```
# Backend
domain/department.go                         # 实体、DTO、接口定义
ent/schema/department.go                     # Ent 数据库 schema
repository/department_repository.go          # 数据访问层
usecase/department_usecase.go                # 业务逻辑层
api/controller/department_controller.go      # HTTP 控制器
api/route/system_routes.go                   # 路由注册（修改）
api/route/factory.go                         # DI 工厂（修改）
bootstrap/admin_init.go                      # 种子菜单数据（修改）

# 数据库 schema 变更
ent/schema/user.go                           # 添加 department edge（修改）

# Frontend
frontend/src/types/department.ts                  # 类型定义
frontend/src/services/departmentApi.ts            # API 服务
frontend/src/features/system/departments/         # 功能模块目录
  ├── index.tsx                              # 页面入口
  ├── components/
  │   ├── departments-table.tsx              # 树形表格组件
  │   ├── departments-dialogs.tsx            # 弹窗管理
  │   ├── departments-action-dialog.tsx      # 新增/编辑弹窗
  │   ├── departments-delete-dialog.tsx      # 删除确认弹窗
  │   └── departments-provider.tsx           # 状态 Provider
  ├── hooks/
  │   └── use-departments.ts                 # TanStack Query hooks
  └── data/
      └── schema.ts                          # Zod 验证 schema
frontend/src/routes/_authenticated/system/departments.tsx  # 路由文件
```

### 2.2 Backend 详细设计

#### 2.2.1 Domain 层 — `domain/department.go`

```go
package domain

// 实体
type Department struct {
    ID        string       `json:"id"`
    ParentID  *string      `json:"parent_id"`
    Name      string       `json:"name"`
    Sequence  int          `json:"sequence"`
    Leader    string       `json:"leader,omitempty"`
    Phone     string       `json:"phone,omitempty"`
    Email     string       `json:"email,omitempty"`
    Status    string       `json:"status"`
    Children  []Department `json:"children,omitempty"`
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
}

// 创建请求
type CreateDepartmentRequest struct {
    ParentID *string `json:"parent_id"`
    Name     string  `json:"name" binding:"required,max=100"`
    Sequence int     `json:"sequence"`
    Leader   string  `json:"leader,omitempty" binding:"max=64"`
    Phone    string  `json:"phone,omitempty" binding:"max=20"`
    Email    string  `json:"email,omitempty" binding:"omitempty,email,max=100"`
    Status   string  `json:"status,omitempty"`
}

// 更新请求（指针字段表示部分更新）
type UpdateDepartmentRequest struct {
    ParentID *string `json:"parent_id,omitempty"`
    Name     *string `json:"name,omitempty" binding:"omitempty,max=100"`
    Sequence *int    `json:"sequence,omitempty"`
    Leader   *string `json:"leader,omitempty" binding:"omitempty,max=64"`
    Phone    *string `json:"phone,omitempty" binding:"omitempty,max=20"`
    Email    *string `json:"email,omitempty" binding:"omitempty,email,max=100"`
    Status   *string `json:"status,omitempty"`
}

// 哨兵错误
var (
    ErrDepartmentHasChildren = errors.New("该部门下存在子部门，无法删除")
    ErrDepartmentHasUsers    = errors.New("该部门下存在用户，无法删除")
    ErrDepartmentNotFound    = errors.New("部门不存在")
    ErrDepartmentNameExists  = errors.New("同级下已存在同名部门")
    ErrCircularDepartment    = errors.New("不能将部门移动到其子部门下")
)

// Repository 接口
type DepartmentRepository interface {
    Create(c context.Context, dept *Department) error
    GetByID(c context.Context, id string) (*Department, error)
    FetchTree(c context.Context) ([]Department, error)
    FetchList(c context.Context, filter DepartmentQueryFilter) ([]Department, error)
    Update(c context.Context, dept *Department) error
    Delete(c context.Context, id string) error
    HasChildren(c context.Context, id string) (bool, error)
    HasUsers(c context.Context, id string) (bool, error)
    GetByNameAndParent(c context.Context, name string, parentID *string) (*Department, error)
    GetAllChildrenIDs(c context.Context, id string) ([]string, error)
}

// UseCase 接口
type DepartmentUseCase interface {
    Create(c context.Context, req *CreateDepartmentRequest) error
    GetByID(c context.Context, id string) (*Department, error)
    FetchTree(c context.Context) ([]Department, error)
    Update(c context.Context, id string, req *UpdateDepartmentRequest) (*Department, error)
    Delete(c context.Context, id string) error
}
```

#### 2.2.2 Ent Schema — `ent/schema/department.go`

```go
// 关键设计点：
// - 自引用 parent/children 关系（参考 Menu schema）
// - 与 User 建立 One-to-Many 关系
// - 索引：parent_id, status, parent_id+sequence, name+parent_id (唯一)
```

#### 2.2.3 User Schema 变更 — `ent/schema/user.go`

```go
// 新增 Edge：
edge.From("department", Department.Type).
    Ref("users").
    Field("department_id").
    Unique().          // 一个用户只属于一个部门
    Comment("所属部门")

// 新增可选字段：
field.String("department_id").
    MaxLen(20).
    Optional().
    Nillable().
    Comment("所属部门ID")
```

> ⚠️ `department_id` 设为 Optional + Nillable，确保现有用户数据无需迁移。

#### 2.2.4 API 路由设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/system/department/tree` | 获取部门树 |
| GET | `/api/v1/system/department` | 获取部门列表（平铺） |
| POST | `/api/v1/system/department` | 创建部门 |
| GET | `/api/v1/system/department/:id` | 获取单个部门 |
| PUT | `/api/v1/system/department/:id` | 更新部门 |
| DELETE | `/api/v1/system/department/:id` | 删除部门 |

所有路由受 JWT + Casbin 保护，挂载在 `systemGroup` 下。

#### 2.2.5 User 相关变更

- `CreateUserRequest` 添加 `DepartmentID *string` 字段
- `UserUpdateRequest` 添加 `DepartmentID *string` 字段
- `User` 实体添加 `DepartmentID *string` 和 `DepartmentName string` 字段
- `UserQueryFilter` 添加 `DepartmentID string` 过滤条件
- 用户列表/详情接口返回中带上部门信息

#### 2.2.6 种子数据 — `bootstrap/admin_init.go`

在 `initMenu()` 中新增：

```go
// 部门管理菜单（系统管理子菜单）
deptMgmt, _ := app.DB.Menu.Create().
    SetParentID(system.ID).SetName("部门管理").SetSequence(7).
    SetType("menu").SetPath("/system/departments").SetIcon("Network").
    SetIsFrame(false).SetVisible("show").
    AddAPIResourceIDs("GET:/api/v1/system/department/tree", "GET:/api/v1/system/department").
    SetStatus("active").Save(ctx)

// 部门管理按钮权限
app.DB.Menu.Create().SetParentID(deptMgmt.ID).SetName("创建部门").SetSequence(0).
    SetType("button").SetPermissions("system:dept:add").
    AddAPIResourceIDs("POST:/api/v1/system/department").SetStatus("active").Save(ctx)
app.DB.Menu.Create().SetParentID(deptMgmt.ID).SetName("编辑部门").SetSequence(1).
    SetType("button").SetPermissions("system:dept:edit").
    AddAPIResourceIDs("PUT:/api/v1/system/department/:id", "GET:/api/v1/system/department/:id").
    SetStatus("active").Save(ctx)
app.DB.Menu.Create().SetParentID(deptMgmt.ID).SetName("删除部门").SetSequence(2).
    SetType("button").SetPermissions("system:dept:delete").
    AddAPIResourceIDs("DELETE:/api/v1/system/department/:id").SetStatus("active").Save(ctx)
```

### 2.3 Frontend 详细设计

#### 2.3.1 类型定义 — `frontend/src/types/department.ts`

```typescript
export interface Department {
  id: string
  parent_id: string | null
  name: string
  sequence: number
  leader: string
  phone: string
  email: string
  status: 'active' | 'inactive'
  children?: Department[]
  created_at: string
  updated_at: string
}

export interface CreateDepartmentRequest {
  parent_id?: string | null
  name: string
  sequence?: number
  leader?: string
  phone?: string
  email?: string
  status?: string
}

export interface UpdateDepartmentRequest {
  parent_id?: string | null
  name?: string
  sequence?: number
  leader?: string
  phone?: string
  email?: string
  status?: string
}
```

#### 2.3.2 页面组件设计

- **树形表格**：使用 TanStack Table 的展开行功能渲染部门树
- **新增/编辑弹窗**：表单包含上级部门（树形选择器）、部门名称、排序序号、负责人、联系电话、邮箱、状态
- **删除确认**：提示"有子部门或用户时无法删除"
- **上级部门选择**：使用树形下拉选择器，编辑时排除自身及其子部门

#### 2.3.3 权限常量

```typescript
// frontend/src/constants/permissions.ts 中新增
DEPARTMENT: {
  READ: 'system:dept:read',
  CREATE: 'system:dept:add',
  EDIT: 'system:dept:edit',
  DELETE: 'system:dept:delete',
}
```

#### 2.3.4 用户管理页面变更

- 用户列表新增"所属部门"列
- 用户新增/编辑弹窗新增"部门"下拉选择器（树形选择）
- 用户筛选支持按部门过滤

---

## 3. 数据库变更评估

### 3.1 新增表

| 表名 | 说明 |
|------|------|
| `departments` | 部门表，自引用 parent_id 实现树形结构 |

### 3.2 修改表

| 表名 | 变更 |
|------|------|
| `users` | 新增 `department_id` 字段 (string, optional, nullable) |

### 3.3 迁移策略

- Ent 自动迁移会新建 `departments` 表并给 `users` 添加 `department_id` 列
- `department_id` 设为 Optional + Nillable，现有用户数据无需修改
- 无破坏性变更，向后兼容

---

## 4. 影响范围分析

### 4.1 新增文件（约 13 个）

| 分类 | 文件 | 复杂度 |
|------|------|--------|
| Backend Domain | `domain/department.go` | 低 |
| Backend Schema | `ent/schema/department.go` | 低 |
| Backend Repository | `repository/department_repository.go` | 中（树形查询） |
| Backend Usecase | `usecase/department_usecase.go` | 中（循环引用检测） |
| Backend Controller | `api/controller/department_controller.go` | 低 |
| Frontend Types | `frontend/src/types/department.ts` | 低 |
| Frontend Service | `frontend/src/services/departmentApi.ts` | 低 |
| Frontend Page | `frontend/src/features/system/departments/index.tsx` | 中 |
| Frontend Components | `departments-table.tsx`, dialogs, provider | 中（树形表格） |
| Frontend Route | `frontend/src/routes/.../departments.tsx` | 低 |

### 4.2 修改文件（约 8 个）

| 文件 | 变更内容 | 风险 |
|------|----------|------|
| `ent/schema/user.go` | 添加 department_id 字段和 department edge | 低（自动迁移） |
| `domain/user.go` | User 实体和 DTO 添加部门字段 | 低 |
| `repository/user_repository.go` | 查询/创建/更新用户时处理 department_id | 低 |
| `usecase/user_usecase.go` | 创建/更新用户时关联部门 | 低 |
| `api/controller/user_controller.go` | 返回部门信息 | 低 |
| `api/route/system_routes.go` | 注册部门路由 | 低 |
| `api/route/factory.go` | 添加 CreateDepartmentController | 低 |
| `bootstrap/admin_init.go` | 添加部门管理菜单种子数据 | 低 |

### 4.3 前端变更

| 文件 | 变更内容 |
|------|----------|
| 用户管理表格 | 新增"部门"列显示 |
| 用户新增/编辑弹窗 | 新增部门选择器 |
| 用户查询筛选 | 新增部门筛选条件 |
| 权限常量 | 新增 DEPARTMENT 权限组 |

---

## 5. 技术难点与注意事项

### 5.1 树形结构操作

| 难点 | 解决方案 |
|------|----------|
| 递归构建树 | Repository 层一次查询所有部门，在内存中递归构建树结构 |
| 防止循环引用 | 更新 parent_id 时，遍历目标父节点的祖先链，检查是否包含自身 |
| 同级名称唯一 | 同一 parent_id 下部门名称不可重复（复合唯一索引） |

### 5.2 删除保护

```
删除前检查：
1. HasChildren(id) → 返回错误 ErrDepartmentHasChildren
2. HasUsers(id)    → 返回错误 ErrDepartmentHasUsers
```

### 5.3 前端树形组件

- 部门树表格：使用 TanStack Table 的 `getExpandedRowModel()` 实现行展开
- 上级部门选择器：需要树形下拉组件，可基于 Shadcn Popover + Command 实现
- 编辑时需排除当前部门及其子部门作为可选父部门（防循环）

---

## 6. 实施步骤建议

### Phase 1：Backend 基础

1. 创建 `domain/department.go` — 实体、DTO、接口
2. 创建 `ent/schema/department.go` — 数据库 schema
3. 修改 `ent/schema/user.go` — 添加部门关系
4. 运行 `go generate ./ent` 生成代码
5. 创建 `repository/department_repository.go`
6. 创建 `usecase/department_usecase.go`

### Phase 2：Backend API

7. 创建 `api/controller/department_controller.go`
8. 修改 `api/route/factory.go` — DI 注册
9. 修改 `api/route/system_routes.go` — 路由注册
10. 修改 `bootstrap/admin_init.go` — 菜单种子数据

### Phase 3：Backend 用户集成

11. 修改 `domain/user.go` — 添加部门字段
12. 修改 `repository/user_repository.go` — 查询带部门
13. 修改 `usecase/user_usecase.go` — 创建/更新用户时关联部门
14. 修改 `api/controller/user_controller.go` — 返回部门信息

### Phase 4：Frontend 部门模块

15. 创建 `frontend/src/types/department.ts`
16. 创建 `frontend/src/services/departmentApi.ts`
17. 创建 `frontend/src/features/system/departments/` 整个模块
18. 创建路由文件

### Phase 5：Frontend 用户集成

19. 修改用户表格、弹窗、筛选器
20. 添加权限常量

### Phase 6：验证与收尾

21. 后端：`go fmt` → `go vet` → `go test`
22. 前端：`pnpm lint` → `pnpm format:check`
23. 重新生成 Swagger 文档
24. 端到端测试

---

## 7. 风险评估

| 风险项 | 级别 | 缓解措施 |
|--------|------|----------|
| 数据库迁移影响现有数据 | 🟢 低 | department_id 为可选字段，无破坏性变更 |
| 树形结构性能（深层嵌套） | 🟢 低 | 实际组织架构层级有限，一次查全部在内存建树 |
| 循环引用导致死循环 | 🟡 中 | Usecase 层做祖先链检查 |
| 种子菜单与已有数据冲突 | 🟡 中 | 已有数据库需手动添加菜单，或提供迁移脚本 |
| 前端树形表格实现复杂度 | 🟡 中 | TanStack Table 原生支持展开行 |
