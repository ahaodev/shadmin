# 开发指南

本文档以添加一个 **"项目管理"（Project）** 模块为例，手把手演示如何在 Shadmin 中开发一个完整的 CRUD 功能——从后端 API 到前端页面。

## 前置要求

- 已完成[快速开始](./quickstart.zh.md)，项目能正常运行
- 已阅读[架构概览](./architecture.zh.md)，理解分层设计
- 熟悉 Go 和 React/TypeScript 基础

---

## 后端开发

### 第 1 步：定义 Domain 层

Domain 是整个模块的契约层，定义实体、DTO、接口和错误常量。

创建 `domain/project.go`：

```go
package domain

import (
	"context"
	"errors"
	"time"
)

// ========== 实体 ==========

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active, archived
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 请求 DTO ==========

// 创建请求：必填字段使用 binding:"required"
type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// 更新请求：可选字段使用指针，区分"未传"和"传空"
type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// 查询参数：嵌入 QueryParams 获得分页能力
type ProjectQueryParams struct {
	QueryParams
	Name   string `form:"name"`
	Code   string `form:"code"`
	Status string `form:"status"`
	Search string `form:"search"`
}

// ========== 接口 ==========

type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
	Fetch(ctx context.Context, params ProjectQueryParams) (*PagedResult[Project], error)
	Update(ctx context.Context, id string, req *UpdateProjectRequest) error
	Delete(ctx context.Context, id string) error
}

type ProjectUseCase interface {
	Create(ctx context.Context, req *CreateProjectRequest) (*Project, error)
	GetByID(ctx context.Context, id string) (*Project, error)
	List(ctx context.Context, params ProjectQueryParams) (*PagedResult[Project], error)
	Update(ctx context.Context, id string, req *UpdateProjectRequest) error
	Delete(ctx context.Context, id string) error
}

// ========== 错误常量 ==========

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrProjectCodeExists = errors.New("project code already exists")
)
```

**要点：**
- `QueryParams` 是内置的分页基础结构（Page, PageSize, SortBy, Order）
- `PagedResult[T]` 是泛型分页响应
- Repository 接口用 `Fetch` 表示分页查询，`GetByID` 表示单条查询
- 更新请求使用指针类型，只更新非 nil 字段

### 第 2 步：创建 Ent Schema

Ent 是 Shadmin 使用的 ORM，通过 Go 代码定义数据库表结构。

创建 `ent/schema/project.go`：

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/rs/xid"
)

type Project struct {
	ent.Schema
}

func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(20).
			NotEmpty().
			Unique().
			Immutable().
			DefaultFunc(func() string {
				return xid.New().String()
			}),
		field.String("name").
			NotEmpty().
			Comment("项目名称"),
		field.String("code").
			NotEmpty().
			Comment("项目编码"),
		field.String("description").
			Default("").
			Comment("项目描述"),
		field.Enum("status").
			Values("active", "archived").
			Default("active").
			Comment("状态"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("status"),
		index.Fields("name"),
	}
}
```

然后生成 ORM 代码：

```bash
go generate ./ent
```

这会在 `ent/` 下生成 `project.go`、`project_create.go`、`project_query.go` 等文件。

**要点：**
- ID 使用 `xid.New().String()` 生成，全局唯一、可排序
- `created_at` 设为 `Immutable()`，创建后不可修改
- `updated_at` 使用 `UpdateDefault(time.Now)` 自动更新
- 枚举字段用 `field.Enum()` 而非字符串，数据库层强约束

### 第 3 步：实现 Repository

Repository 负责数据存取，是 Domain 接口的具体实现。

创建 `repository/project_repository.go`：

```go
package repository

import (
	"context"
	"math"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/project"
)

type projectRepository struct {
	client *ent.Client
}

func NewProjectRepository(client *ent.Client) domain.ProjectRepository {
	return &projectRepository{client: client}
}

func (r *projectRepository) Create(ctx context.Context, p *domain.Project) error {
	// 检查编码唯一性
	exists, _ := r.client.Project.Query().
		Where(project.Code(p.Code)).
		Exist(ctx)
	if exists {
		return domain.ErrProjectCodeExists
	}

	result, err := r.client.Project.Create().
		SetName(p.Name).
		SetCode(p.Code).
		SetDescription(p.Description).
		SetStatus(project.Status(p.Status)).
		Save(ctx)
	if err != nil {
		return err
	}

	p.ID = result.ID
	p.CreatedAt = result.CreatedAt
	p.UpdatedAt = result.UpdatedAt
	return nil
}

func (r *projectRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	result, err := r.client.Project.Query().
		Where(project.ID(id)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, err
	}
	return r.toDomain(result), nil
}

func (r *projectRepository) Fetch(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[domain.Project], error) {
	query := r.client.Project.Query()

	// 应用过滤条件
	if params.Name != "" {
		query = query.Where(project.NameContains(params.Name))
	}
	if params.Code != "" {
		query = query.Where(project.CodeContains(params.Code))
	}
	if params.Status != "" {
		query = query.Where(project.StatusEQ(project.Status(params.Status)))
	}
	if params.Search != "" {
		query = query.Where(
			project.Or(
				project.NameContains(params.Search),
				project.CodeContains(params.Search),
			),
		)
	}

	// 获取总数（克隆查询）
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	// 排序
	if params.SortBy != "" {
		if params.Order == "desc" {
			query = query.Order(ent.Desc(params.SortBy))
		} else {
			query = query.Order(ent.Asc(params.SortBy))
		}
	} else {
		query = query.Order(ent.Desc("created_at"))
	}

	// 分页
	offset := (params.Page - 1) * params.PageSize
	results, err := query.Offset(offset).Limit(params.PageSize).All(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 Domain 实体
	list := make([]domain.Project, 0, len(results))
	for _, item := range results {
		list = append(list, *r.toDomain(item))
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &domain.PagedResult[domain.Project]{
		List:       list,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *projectRepository) Update(ctx context.Context, id string, req *domain.UpdateProjectRequest) error {
	// 检查记录存在
	_, err := r.client.Project.Query().
		Where(project.ID(id)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrProjectNotFound
		}
		return err
	}

	// 如果要修改编码，检查新编码唯一性
	if req.Code != nil {
		exists, _ := r.client.Project.Query().
			Where(project.Code(*req.Code), project.IDNEQ(id)).
			Exist(ctx)
		if exists {
			return domain.ErrProjectCodeExists
		}
	}

	update := r.client.Project.UpdateOneID(id)
	if req.Name != nil {
		update = update.SetName(*req.Name)
	}
	if req.Code != nil {
		update = update.SetCode(*req.Code)
	}
	if req.Description != nil {
		update = update.SetDescription(*req.Description)
	}
	if req.Status != nil {
		update = update.SetStatus(project.Status(*req.Status))
	}

	return update.Exec(ctx)
}

func (r *projectRepository) Delete(ctx context.Context, id string) error {
	err := r.client.Project.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrProjectNotFound
		}
		return err
	}
	return nil
}

// Ent 实体 → Domain 实体转换
func (r *projectRepository) toDomain(e *ent.Project) *domain.Project {
	return &domain.Project{
		ID:          e.ID,
		Name:        e.Name,
		Code:        e.Code,
		Description: e.Description,
		Status:      string(e.Status),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
```

**要点：**
- 构造函数返回接口类型（`domain.ProjectRepository`），符合依赖倒置
- 分页查询先 `Clone()` 获取总数，再排序+分页
- 更新时只设置非 nil 的字段
- 使用 `ent.IsNotFound()` 判断记录不存在
- `toDomain` 辅助方法做 Ent 实体到 Domain 实体的转换

### 第 4 步：实现 Usecase

Usecase 编排业务逻辑，是 Controller 和 Repository 之间的桥梁。

创建 `usecase/project_usecase.go`：

```go
package usecase

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"time"
)

type projectUsecase struct {
	client            *ent.Client
	projectRepository domain.ProjectRepository
	contextTimeout    time.Duration
}

func NewProjectUsecase(
	client *ent.Client,
	projectRepository domain.ProjectRepository,
	timeout time.Duration,
) domain.ProjectUseCase {
	return &projectUsecase{
		client:            client,
		projectRepository: projectRepository,
		contextTimeout:    timeout,
	}
}

func (u *projectUsecase) Create(ctx context.Context, req *domain.CreateProjectRequest) (*domain.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// 设置默认值
	if req.Status == "" {
		req.Status = "active"
	}

	// 校验枚举值
	if req.Status != "active" && req.Status != "archived" {
		return nil, fmt.Errorf("invalid status: must be 'active' or 'archived'")
	}

	p := &domain.Project{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Status:      req.Status,
	}

	if err := u.projectRepository.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *projectUsecase) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.projectRepository.GetByID(ctx, id)
}

func (u *projectUsecase) List(ctx context.Context, params domain.ProjectQueryParams) (*domain.PagedResult[domain.Project], error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	domain.ValidateQueryParams(&params.QueryParams)
	return u.projectRepository.Fetch(ctx, params)
}

func (u *projectUsecase) Update(ctx context.Context, id string, req *domain.UpdateProjectRequest) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	// 校验枚举值
	if req.Status != nil && *req.Status != "active" && *req.Status != "archived" {
		return fmt.Errorf("invalid status: must be 'active' or 'archived'")
	}

	return u.projectRepository.Update(ctx, id, req)
}

func (u *projectUsecase) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	return u.projectRepository.Delete(ctx, id)
}
```

**要点：**
- 每个方法都用 `context.WithTimeout` 包装上下文，防止长时间阻塞
- `ValidateQueryParams` 校验并修正分页参数（Page < 1 设为 1，PageSize 超限截断）
- Usecase 不直接操作数据库，只调用 Repository 接口
- 业务校验（枚举值、默认值）放在 Usecase 层

### 第 5 步：编写 Controller

Controller 是 HTTP 处理器，负责请求解析、响应格式化和 Swagger 文档。

创建 `api/controller/project_controller.go`：

```go
package controller

import (
	"net/http"
	"shadmin/domain"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProjectController struct {
	ProjectUseCase domain.ProjectUseCase
}

// @Summary     获取项目列表
// @Description 分页获取项目列表，支持搜索和筛选
// @Tags        项目管理
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       page      query    int    false  "页码"
// @Param       page_size query    int    false  "每页数量"
// @Param       name      query    string false  "项目名称"
// @Param       code      query    string false  "项目编码"
// @Param       status    query    string false  "状态"
// @Param       search    query    string false  "搜索关键词"
// @Param       sort_by   query    string false  "排序字段"
// @Param       order     query    string false  "排序方向"
// @Success     200       {object} domain.Response
// @Failure     500       {object} domain.Response
// @Router      /system/projects [get]
func (pc *ProjectController) List(c *gin.Context) {
	var params domain.ProjectQueryParams

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			params.Page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			params.PageSize = v
		}
	}
	params.Name = c.Query("name")
	params.Code = c.Query("code")
	params.Status = c.Query("status")
	params.Search = c.Query("search")
	params.SortBy = c.Query("sort_by")
	params.Order = c.Query("order")

	result, err := pc.ProjectUseCase.List(c, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// @Summary     获取项目详情
// @Description 根据 ID 获取项目详情
// @Tags        项目管理
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "项目 ID"
// @Success     200 {object} domain.Response
// @Failure     404 {object} domain.Response
// @Router      /system/projects/{id} [get]
func (pc *ProjectController) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := pc.ProjectUseCase.GetByID(c, id)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// @Summary     创建项目
// @Description 创建一个新项目
// @Tags        项目管理
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body     domain.CreateProjectRequest true "创建项目请求"
// @Success     201  {object} domain.Response
// @Failure     400  {object} domain.Response
// @Failure     409  {object} domain.Response
// @Router      /system/projects [post]
func (pc *ProjectController) Create(c *gin.Context) {
	var req domain.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	result, err := pc.ProjectUseCase.Create(c, &req)
	if err != nil {
		if err == domain.ErrProjectCodeExists {
			c.JSON(http.StatusConflict, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, domain.RespSuccess(result))
}

// @Summary     更新项目
// @Description 更新项目信息
// @Tags        项目管理
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path     string                     true "项目 ID"
// @Param       body body     domain.UpdateProjectRequest true "更新项目请求"
// @Success     200  {object} domain.Response
// @Failure     400  {object} domain.Response
// @Failure     404  {object} domain.Response
// @Failure     409  {object} domain.Response
// @Router      /system/projects/{id} [put]
func (pc *ProjectController) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	err := pc.ProjectUseCase.Update(c, id, &req)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		if err == domain.ErrProjectCodeExists {
			c.JSON(http.StatusConflict, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(nil))
}

// @Summary     删除项目
// @Description 删除指定项目
// @Tags        项目管理
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "项目 ID"
// @Success     200 {object} domain.Response
// @Failure     404 {object} domain.Response
// @Router      /system/projects/{id} [delete]
func (pc *ProjectController) Delete(c *gin.Context) {
	id := c.Param("id")

	err := pc.ProjectUseCase.Delete(c, id)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			c.JSON(http.StatusNotFound, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(nil))
}
```

**要点：**
- Query 参数手动解析（`c.Query` + `strconv.Atoi`），解析失败静默忽略
- Body 参数使用 `c.ShouldBindJSON`，Gin 自动校验 `binding:"required"`
- 路径参数使用 `c.Param("id")`
- 响应使用 `domain.RespSuccess(data)` / `domain.RespError(msg)` 统一格式
- Domain 错误映射为对应 HTTP 状态码（404、409 等）
- Swagger 注解写在方法上方，`@Security BearerAuth` 表示需要认证

### 第 6 步：注册路由

创建 `api/route/project_routes.go`：

```go
package route

import (
	"shadmin/api/middleware"

	"github.com/gin-gonic/gin"
)

func (pr *ProtectedRoutes) setupProjectManagement(
	systemGroup *gin.RouterGroup,
	casbinMiddleware *middleware.CasbinMiddleware,
) {
	projectGroup := systemGroup.Group("/projects")
	projectGroup.Use(casbinMiddleware.CheckAPIPermission())
	projectController := pr.factory.CreateProjectController()

	projectGroup.GET("", projectController.List)
	projectGroup.POST("", projectController.Create)
	projectGroup.GET("/:id", projectController.GetByID)
	projectGroup.PUT("/:id", projectController.Update)
	projectGroup.DELETE("/:id", projectController.Delete)
}
```

然后在 `api/route/system_routes.go` 的 `SetupSystemRoutes` 方法中调用：

```go
func (pr *ProtectedRoutes) SetupSystemRoutes(...) {
    systemGroup := ...
    // 已有的路由
    pr.setupUserManagement(systemGroup, casbinMiddleware)
    pr.setupRoleManagement(systemGroup, casbinMiddleware)
    // ...

    // 添加新模块
    pr.setupProjectManagement(systemGroup, casbinMiddleware)
}
```

**要点：**
- 每个模块独立一个路由文件，方法挂在 `ProtectedRoutes` 上
- `casbinMiddleware.CheckAPIPermission()` 自动检查 API 级别权限
- RESTful 风格：GET 列表、POST 创建、GET/:id 详情、PUT/:id 更新、DELETE/:id 删除

### 第 7 步：接入工厂

在 `api/route/factory.go` 中添加构造方法：

```go
func (f *ControllerFactory) CreateProjectController() *controller.ProjectController {
	projectRepository := repository.NewProjectRepository(f.db)
	projectUseCase := usecase.NewProjectUsecase(f.db, projectRepository, f.timeout)
	return &controller.ProjectController{ProjectUseCase: projectUseCase}
}
```

工厂负责组装依赖链：`Repository → Usecase → Controller`。这是 Shadmin 的手动 DI 方式，不使用框架。

### 第 8 步：生成 Swagger 文档

```bash
# 安装 swag（如未安装）
go install github.com/swaggo/swag/cmd/swag@latest

# 生成文档
swag init -g main.go --output ./docs

# 启动项目后访问 Swagger UI
# http://localhost:55667/swagger/index.html
```

至此，后端开发完成。启动项目后可用 Swagger 或 curl 测试 API：

```bash
# 登录获取 token
curl -X POST http://localhost:55667/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 创建项目（替换 YOUR_TOKEN）
curl -X POST http://localhost:55667/api/v1/system/projects \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试项目","code":"test-001"}'
```

---

## 前端开发

### 第 1 步：定义类型与 API

#### 定义类型

创建 `web/src/types/project.ts`：

```typescript
// 状态类型
export type ProjectStatus = 'active' | 'archived'

// 实体接口
export interface Project {
  id: string
  name: string
  code: string
  description: string
  status: ProjectStatus
  created_at: Date
  updated_at: Date
}

// 创建请求
export interface CreateProjectRequest {
  name: string
  code: string
  description?: string
  status?: ProjectStatus
}

// 更新请求
export interface UpdateProjectRequest {
  name?: string
  code?: string
  description?: string
  status?: ProjectStatus
}

// 查询参数
export interface ProjectQueryParams {
  page?: number
  page_size?: number
  name?: string
  code?: string
  status?: ProjectStatus
  search?: string
  sort_by?: string
  order?: 'asc' | 'desc'
}

// 分页结果
export interface ProjectPagedResult {
  list: Project[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
```

#### 定义 API 服务

创建 `web/src/services/projectApi.ts`：

```typescript
import { apiClient } from './config'
import type {
  Project,
  CreateProjectRequest,
  UpdateProjectRequest,
  ProjectQueryParams,
  ProjectPagedResult,
} from '@/types/project'

// 日期字段解析
const parseProject = (p: any): Project => ({
  ...p,
  created_at: new Date(p.created_at),
  updated_at: new Date(p.updated_at),
})

// GET /system/projects
export const getProjects = async (params?: ProjectQueryParams): Promise<ProjectPagedResult> => {
  const searchParams = new URLSearchParams()
  if (params?.page) searchParams.append('page', params.page.toString())
  if (params?.page_size) searchParams.append('page_size', params.page_size.toString())
  if (params?.name) searchParams.append('name', params.name)
  if (params?.code) searchParams.append('code', params.code)
  if (params?.status) searchParams.append('status', params.status)
  if (params?.search) searchParams.append('search', params.search)
  if (params?.sort_by) searchParams.append('sort_by', params.sort_by)
  if (params?.order) searchParams.append('order', params.order)

  const response = await apiClient.get(`/api/v1/system/projects?${searchParams}`)
  const data = response.data.data as ProjectPagedResult
  return {
    ...data,
    list: (data.list || []).map(parseProject),
  }
}

// GET /system/projects/:id
export const getProject = async (id: string): Promise<Project> => {
  const response = await apiClient.get(`/api/v1/system/projects/${id}`)
  return parseProject(response.data.data)
}

// POST /system/projects
export const createProject = async (data: CreateProjectRequest): Promise<Project> => {
  const response = await apiClient.post('/api/v1/system/projects', data)
  return parseProject(response.data.data)
}

// PUT /system/projects/:id
export const updateProject = async (id: string, data: UpdateProjectRequest): Promise<void> => {
  await apiClient.put(`/api/v1/system/projects/${id}`, data)
}

// DELETE /system/projects/:id
export const deleteProject = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/projects/${id}`)
}
```

**要点：**
- `apiClient` 从 `./config` 导入，已配置 baseURL 和 JWT 拦截器
- 响应数据路径是 `response.data.data`（外层 `.data` 是 Axios，内层 `.data` 是 `domain.Response.Data`）
- 日期字段需要手动 `new Date()` 转换

### 第 2 步：创建功能模块

在 `web/src/features/system/` 下创建项目模块目录结构：

```
web/src/features/system/projects/
├── components/
│   ├── projects-provider.tsx     # Context Provider（状态管理）
│   ├── projects-columns.tsx      # 表格列定义
│   ├── projects-table.tsx        # 数据表格
│   ├── projects-dialogs.tsx      # 弹窗汇总
│   ├── projects-primary-buttons.tsx  # 头部操作按钮
│   └── project-form-dialog.tsx   # 创建/编辑表单弹窗
├── data/
│   └── schema.ts                 # Zod 校验 Schema
├── hooks/
│   └── use-projects.ts           # TanStack Query Hooks
└── index.tsx                     # 页面入口
```

#### Context Provider

创建 `web/src/features/system/projects/components/projects-provider.tsx`：

```tsx
import { createContext, useContext, useMemo, useState, type ReactNode, type Dispatch, type SetStateAction } from 'react'
import type { Project } from '@/types/project'

interface ProjectsContext {
  currentRow: Project | null
  setCurrentRow: Dispatch<SetStateAction<Project | null>>
  showCreateDialog: boolean
  setShowCreateDialog: Dispatch<SetStateAction<boolean>>
  showEditDialog: boolean
  setShowEditDialog: Dispatch<SetStateAction<boolean>>
  showDeleteDialog: boolean
  setShowDeleteDialog: Dispatch<SetStateAction<boolean>>
}

const ProjectsContext = createContext<ProjectsContext | null>(null)

export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [currentRow, setCurrentRow] = useState<Project | null>(null)
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [showEditDialog, setShowEditDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const value: ProjectsContext = useMemo(() => ({
    currentRow, setCurrentRow,
    showCreateDialog, setShowCreateDialog,
    showEditDialog, setShowEditDialog,
    showDeleteDialog, setShowDeleteDialog,
  }), [currentRow, showCreateDialog, showEditDialog, showDeleteDialog])

  return (
    <ProjectsContext.Provider value={value}>
      {children}
    </ProjectsContext.Provider>
  )
}

export const useProjects = () => {
  const ctx = useContext(ProjectsContext)
  if (!ctx) throw new Error('useProjects must be used within <ProjectsProvider>')
  return ctx
}
```

**要点：**
- 每个功能模块用 Context 管理弹窗状态和当前行数据
- `useMemo` 优化重渲染
- 自定义 Hook `useProjects()` 封装 Context 消费

### 第 3 步：编写 Hooks

创建 `web/src/features/system/projects/hooks/use-projects.ts`：

```typescript
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { ProjectQueryParams } from '@/types/project'
import { deleteProject, getProjects } from '@/services/projectApi'

const PROJECTS_QUERY_KEY = 'projects'

// 列表查询
export function useProjectList(params?: ProjectQueryParams) {
  return useQuery({
    queryKey: [PROJECTS_QUERY_KEY, params],
    queryFn: () => getProjects(params),
    staleTime: 5 * 60 * 1000,
  })
}

// 删除
export function useDeleteProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [PROJECTS_QUERY_KEY] })
      toast.success('删除成功')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || '删除失败')
    },
  })
}
```

**要点：**
- 查询 Key 使用字符串常量 + 参数对象，TanStack Query 自动缓存和失效
- `staleTime` 控制缓存时间
- Mutation 成功后 `invalidateQueries` 刷新列表
- 错误提示从后端响应的 `msg` 字段取

### 第 4 步：实现列表页

创建 `web/src/features/system/projects/index.tsx`：

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Skeleton } from '@/components/ui/skeleton'
import { ProjectsProvider } from './components/projects-provider'
import { ProjectsPrimaryButtons } from './components/projects-primary-buttons'
import { ProjectsTable } from './components/projects-table'
import { ProjectsDialogs } from './components/projects-dialogs'
import { useProjectList } from './hooks/use-projects'

const route = getRouteApi('/_authenticated/system/projects')

export function Projects() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const queryParams = {
    page: search.page || 1,
    page_size: search.page_size || 10,
    search: search.search || undefined,
    status: search.status || undefined,
  }

  const { data, isLoading, error } = useProjectList(queryParams)

  return (
    <ProjectsProvider>
      <Header fixed>
        <Search />
        <div className='ms-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 space-y-2'>
          <div className='flex items-center justify-between'>
            <h2 className='text-2xl font-bold tracking-tight'>项目管理</h2>
            <ProjectsPrimaryButtons />
          </div>
          <p className='text-muted-foreground'>管理项目信息。</p>
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isLoading ? (
            <div className='space-y-4'>
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className='h-16 w-full' />
              ))}
            </div>
          ) : error ? (
            <div className='flex h-32 items-center justify-center text-muted-foreground'>
              加载失败，请重试
            </div>
          ) : (
            <ProjectsTable
              data={data?.list || []}
              search={search}
              navigate={(opts) => navigate(opts)}
              totalCount={data?.total || 0}
            />
          )}
        </div>
      </Main>

      <ProjectsDialogs />
    </ProjectsProvider>
  )
}
```

#### 补充组件：表格列定义

创建 `web/src/features/system/projects/components/projects-columns.tsx`：

```tsx
import { ColumnDef } from '@tanstack/react-table'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { DataTableRowActions } from '@/components/data-table/data-table-row-actions'
import { LongText } from '@/components/long-text'
import { Project } from '@/types/project'
import { useProjects } from './projects-provider'

export function useProjectColumns(): ColumnDef<Project>[] {
  const { setOpen, setCurrentRow } = useProjects()

  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label='Select row'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'name',
      header: '项目名称',
      cell: ({ row }) => <LongText className='max-w-36'>{row.getValue('name')}</LongText>,
    },
    {
      accessorKey: 'code',
      header: '项目编码',
    },
    {
      accessorKey: 'status',
      header: '状态',
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return <Badge variant={status === 'active' ? 'default' : 'secondary'}>{status}</Badge>
      },
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
    },
    {
      accessorKey: 'created_at',
      header: '创建时间',
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <DataTableRowActions
          row={row}
          onEdit={() => { setCurrentRow(row.original); setOpen('edit') }}
          onDelete={() => { setCurrentRow(row.original); setOpen('delete') }}
        />
      ),
    },
  ]
}
```

#### 补充组件：数据表格

创建 `web/src/features/system/projects/components/projects-table.tsx`：

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { useReactTable, getCoreRowModel, getPaginationRowModel } from '@tanstack/react-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTable } from '@/components/data-table/data-table'
import { DataTablePagination } from '@/components/data-table/data-table-pagination'
import { Project } from '@/types/project'
import { useProjectColumns } from './projects-columns'

const route = getRouteApi('/_authenticated/system/projects')

interface ProjectsTableProps {
  data: Project[]
  search: Record<string, unknown>
  navigate: (opts: any) => void
  totalCount: number
}

export function ProjectsTable({ data, search, navigate, totalCount }: ProjectsTableProps) {
  const columns = useProjectColumns()

  const { pagination, onPaginationChange, columnFilters, onColumnFiltersChange } =
    useTableUrlState({ search, navigate })

  const table = useReactTable({
    data,
    columns,
    pageCount: Math.ceil(totalCount / (pagination.pageSize || 10)),
    state: { pagination, columnFilters },
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  return (
    <div className='space-y-4'>
      <DataTable table={table} />
      <DataTablePagination table={table} />
    </div>
  )
}
```

#### 补充组件：弹窗汇总

创建 `web/src/features/system/projects/components/projects-dialogs.tsx`：

```tsx
import { useProjects } from './projects-provider'
import { ProjectFormDialog } from './project-form-dialog'

export function ProjectsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useProjects()

  return (
    <>
      <ProjectFormDialog
        key='project-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
      />
      {currentRow && (
        <ProjectFormDialog
          key={`project-edit-${currentRow.id}`}
          open={open === 'edit'}
          onOpenChange={() => {
            setOpen('edit')
            setTimeout(() => setCurrentRow(null), 500)
          }}
          currentRow={currentRow}
        />
      )}
    </>
  )
}
```

#### 补充组件：头部操作按钮

创建 `web/src/features/system/projects/components/projects-primary-buttons.tsx`：

```tsx
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermission } from '@/hooks/use-permission'
import { PERMISSIONS } from '@/constants/permissions'
import { useProjects } from './projects-provider'

export function ProjectsPrimaryButtons() {
  const { setOpen } = useProjects()
  const { hasPermission } = usePermission()

  return (
    <div className='flex gap-2'>
      {hasPermission(PERMISSIONS.SYSTEM.PROJECT.ADD) && (
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>添加项目</span> <Plus size={18} />
        </Button>
      )}
    </div>
  )
}
```

> **要点：** 这四个组件遵循固定模式——可直接参考 `web/src/features/system/users/components/` 目录下的对应文件，按相同结构替换实体名称即可。

### 第 5 步：实现表单组件

创建 `web/src/features/system/projects/data/schema.ts`：

```typescript
import { z } from 'zod'

export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
  code: z.string(),
  description: z.string(),
  status: z.enum(['active', 'archived']),
  created_at: z.date(),
  updated_at: z.date(),
})

export type ProjectSchema = z.infer<typeof projectSchema>
```

创建 `web/src/features/system/projects/components/project-form-dialog.tsx`：

```tsx
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import {
  Dialog, DialogClose, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { createProject, updateProject } from '@/services/projectApi'
import { useProjects } from './projects-provider'
import { toast } from 'sonner'

const formSchema = z.object({
  name: z.string().min(1, '请输入项目名称'),
  code: z.string().min(1, '请输入项目编码'),
  description: z.string(),
  status: z.enum(['active', 'archived']),
})

type ProjectForm = z.infer<typeof formSchema>

export function ProjectFormDialog({ mode }: { mode: 'create' | 'edit' }) {
  const {
    currentRow,
    showCreateDialog, setShowCreateDialog,
    showEditDialog, setShowEditDialog,
    setCurrentRow,
  } = useProjects()

  const isOpen = mode === 'create' ? showCreateDialog : showEditDialog
  const setOpen = mode === 'create' ? setShowCreateDialog : setShowEditDialog

  const queryClient = useQueryClient()

  const form = useForm<ProjectForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { name: '', code: '', description: '', status: 'active' },
  })

  // 编辑时填充表单
  useEffect(() => {
    if (mode === 'edit' && currentRow && showEditDialog) {
      form.reset({
        name: currentRow.name,
        code: currentRow.code,
        description: currentRow.description || '',
        status: currentRow.status as 'active' | 'archived',
      })
    }
  }, [currentRow, showEditDialog, mode, form])

  const mutation = useMutation({
    mutationFn: async (values: ProjectForm) => {
      if (mode === 'create') {
        return createProject(values)
      } else {
        return updateProject(currentRow!.id, values)
      }
    },
    onSuccess: () => {
      setOpen(false)
      form.reset()
      if (mode === 'edit') setCurrentRow(null)
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      toast.success(mode === 'create' ? '创建成功' : '更新成功')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || (mode === 'create' ? '创建失败' : '更新失败'))
    },
  })

  const handleOpenChange = (open: boolean) => {
    setOpen(open)
    if (!open) {
      form.reset()
      if (mode === 'edit') setCurrentRow(null)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? '创建项目' : '编辑项目'}</DialogTitle>
          <DialogDescription>
            {mode === 'create' ? '创建一个新项目' : '修改项目信息'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit((v) => mutation.mutateAsync(v))} className='space-y-4'>
            <FormField control={form.control} name='name' render={({ field }) => (
              <FormItem>
                <FormLabel>项目名称 *</FormLabel>
                <FormControl><Input placeholder='请输入项目名称' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='code' render={({ field }) => (
              <FormItem>
                <FormLabel>项目编码 *</FormLabel>
                <FormControl><Input placeholder='请输入项目编码' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='status' render={({ field }) => (
              <FormItem>
                <FormLabel>状态</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                  <SelectContent>
                    <SelectItem value='active'>启用</SelectItem>
                    <SelectItem value='archived'>归档</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name='description' render={({ field }) => (
              <FormItem>
                <FormLabel>描述</FormLabel>
                <FormControl><Textarea placeholder='请输入项目描述' {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <DialogFooter>
              <DialogClose asChild>
                <Button variant='outline' disabled={mutation.isPending}>取消</Button>
              </DialogClose>
              <Button type='submit' disabled={mutation.isPending}>
                {mutation.isPending ? '提交中...' : '确认'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

**要点：**
- 使用 React Hook Form + Zod 做表单校验
- `useEffect` 在编辑模式时用当前行数据填充表单
- Mutation 成功后 `invalidateQueries` 刷新列表 + `toast` 提示
- 创建和编辑复用同一个表单组件，通过 `mode` 区分

### 第 6 步：注册路由

创建 `web/src/routes/_authenticated/system/projects.tsx`：

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Projects } from '@/features/system/projects'

const searchSchema = z.object({
  page: z.number().min(1).optional().default(1),
  page_size: z.number().min(1).max(100).optional().default(20),
  status: z.enum(['active', 'archived']).optional(),
  search: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/system/projects')({
  component: Projects,
  validateSearch: searchSchema,
})
```

**要点：**
- TanStack Router 使用文件路由，文件路径即 URL 路径
- `validateSearch` 用 Zod 校验 URL 搜索参数，自动提供类型安全
- `/_authenticated/` 前缀自动应用 JWT 路由守卫

创建路由文件后，执行以下命令重新生成路由树：

```bash
cd web && pnpm run dev
# TanStack Router 会自动重新生成 routeTree.gen.ts
```

### 第 7 步：添加菜单与权限

#### 注册权限常量

在 `web/src/constants/permissions.ts` 中添加：

```typescript
export const PERMISSIONS = {
  SYSTEM: {
    // ... 已有的权限
    PROJECT: {
      ADD: 'system:project:add',
      EDIT: 'system:project:edit',
      DELETE: 'system:project:delete',
    },
  },
} as const
```

#### 在后台添加菜单

登录管理后台，进入 **系统管理 → 菜单管理**：

1. **添加菜单项**：名称"项目管理"，路径 `/system/projects`，父级选"系统管理"
2. **分配权限按钮**：为菜单添加按钮权限（新增、编辑、删除），权限标识与 `PERMISSIONS` 常量对应
3. **分配角色**：在 **角色管理** 中将新菜单分配给对应角色

#### 在后台添加 API 资源

进入 **系统管理 → API 资源管理**，重新扫描路由。系统会自动发现新增的 API 路由（格式 `GET:/api/v1/system/projects` 等），然后将它们分配给对应角色。

---

## 常见模式与技巧

### 文件上传

后端使用 `c.FormFile("file")` 接收文件，调用 `pkg/storage` 的存储接口保存：

```go
file, err := c.FormFile("file")
if err != nil {
    c.JSON(http.StatusBadRequest, domain.RespError("文件上传失败"))
    return
}
path, err := storage.Upload(file)
```

前端使用 `FormData` 上传：

```typescript
const formData = new FormData()
formData.append('file', file)
await apiClient.post('/api/v1/upload', formData, {
  headers: { 'Content-Type': 'multipart/form-data' },
})
```

### 字典选择

前端使用 `getDictItemsByTypeCode` 获取字典项作为选择列表：

```typescript
import { getDictItemsByTypeCode } from '@/services/dictApi'

const statusOptions = await getDictItemsByTypeCode('project_status')
// 返回 [{label: "进行中", value: "active"}, ...]
```

### 树形数据

参考菜单模块（`domain/menu.go`），使用 `parent_id` 字段构建树形结构，前端递归渲染。

### 批量操作

参考字典模块的 `useDeleteDictTypes` Hook，使用 `Promise.all` 并发处理：

```typescript
export function useBatchDelete() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (ids: string[]) => {
      return Promise.all(ids.map(id => deleteProject(id)))
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      toast.success('批量删除成功')
    },
  })
}
```

---

## 质量保障

### 后端

```bash
# 编译检查
go build ./...

# 静态分析
go vet ./...

# 代码格式化
go fmt ./...

# 单元测试
go test ./...

# 测试覆盖率
go test ./... -cover
```

### 前端

```bash
cd web

# TypeScript 类型检查
tsc -b

# ESLint 检查
pnpm run lint

# 代码格式化检查
pnpm run format:check

# 格式化（自动修复）
pnpm run format

# 未使用依赖检查
pnpm run knip
```

---

## 常见问题排查

### 编译错误：`ent/client.go` 报错

运行 `go generate ./ent` 重新生成 ORM 代码。确保 Ent Schema 中的字段类型与 Domain 实体一致。

### 权限不生效

1. 确认 API 资源已在后台扫描并分配给角色
2. 确认 Casbin 策略已更新（重启服务或调用刷新接口）
3. 检查路由是否添加了 `casbinMiddleware.CheckAPIPermission()`

**调试步骤：**

```bash
# 1. 用 curl 测试接口，观察返回的 HTTP 状态码
# 401 = JWT Token 无效/过期；403 = Casbin 权限不足
curl -v -H "Authorization: Bearer YOUR_TOKEN" http://localhost:55667/api/v1/system/projects

# 2. 登录管理后台 → API 资源管理，确认新接口已出现在列表中
# 如果没有，重启后端服务触发 bootstrap.InitApiResources() 自动扫描

# 3. 进入角色管理 → 编辑角色 → 勾选新的 API 资源和菜单
# 保存后 Casbin 策略会自动更新

# 4. 重新登录或刷新 Token，使新权限生效
```

> **权限模型说明：** Shadmin 使用双层权限：后端通过 Casbin 按 `(userID, path, method)` 控制 API 访问；前端通过 `PERMISSIONS` 常量字符串（如 `system:project:add`）控制按钮/菜单显示。两者通过"角色 → 菜单 → API 资源"的绑定关系关联——给角色分配菜单时，同时分配该菜单下的 API 资源权限。

### 前端路由 404

1. 确认路由文件在 `web/src/routes/_authenticated/` 下
2. 检查 `routeTree.gen.ts` 是否已自动更新（需重启 `pnpm dev`）
3. 确认 `createFileRoute` 的路径字符串与文件位置一致

### Swagger 文档未更新

重新运行 `swag init -g main.go --output ./docs`，确保 Controller 方法上的 `@Router` 注解路径正确。

### 前端请求 401

检查 JWT Token 是否过期。Access Token 默认 3 小时过期，可通过 `.env` 中的 `ACCESS_TOKEN_EXPIRY_HOUR` 调整。
