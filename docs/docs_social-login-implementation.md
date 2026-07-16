# shadmin 第三方登录接入方案（基于 goth）

## 1. 目标

把第三方登录接到现有的 `shadmin` 认证体系中，而不是引入一套新的登录系统。

最终效果：

- 用户点击“Google / GitHub " 登录”
- 跳转到对应 provider
- 回调后拿到用户 profile
- 在 shadmin 中查找/创建用户
- 绑定第三方账号
- 复用现有 JWT 体系（access token + refresh token）
- 前端拿到 token 后进入系统

> 说明：
> - Google / GitHub：可以直接用 `goth` 官方 provider
> - WeChat / Alipay：通常需要第三方 provider 包，或者自己实现一个 provider

---

## 2. 架构

保持 Clean Architecture 风格：

- `api/controller`：HTTP 入口
- `usecase`：OAuth 业务逻辑
- `repository`：用户/第三方账号数据库操作
- `domain`：实体和接口定义
- `bootstrap`：provider 注册初始化

最终逻辑为：

1. 前端发起第三方登录
2. 后端跳转到 provider
3. provider 回调
4. 解析 profile
5. 查/建用户
6. 绑定第三方账号
7. 生成 shadmin JWT
8. 返回前端 token / redirect 到前端页面

---

## 3. 环境变量

建议在 `.env.example` 中追加：

```env
# Social login
SOCIAL_BASE_URL=http://localhost:55667
SOCIAL_REDIRECT_URL=http://localhost:5173/oauth/callback

GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=

```

---

## 4. 初始化 provider（bootstrap）

新增文件：`bootstrap/social_auth.go`

```go

func InitSocialProviders(baseURL string) {
	// Google
	if os.Getenv("GOOGLE_CLIENT_ID") != "" && os.Getenv("GOOGLE_CLIENT_SECRET") != "" {
		goth.UseProviders(
			google.New(
				os.Getenv("GOOGLE_CLIENT_ID"),
				os.Getenv("GOOGLE_CLIENT_SECRET"),
				fmt.Sprintf("%s/api/v1/auth/social/google/callback", baseURL),
			),
		)
	}

	// GitHub
	if os.Getenv("GITHUB_CLIENT_ID") != "" && os.Getenv("GITHUB_CLIENT_SECRET") != "" {
		goth.UseProviders(
			github.New(
				os.Getenv("GITHUB_CLIENT_ID"),
				os.Getenv("GITHUB_CLIENT_SECRET"),
				fmt.Sprintf("%s/api/v1/auth/social/github/callback", baseURL),
			),
		)
	}

	// WeChat / Alipay can be added later
	// Example:
	// wechat.New(...)
	// alipay.New(...)
}
```

然后在应用启动时调用：

```go
// bootstrap/app.go 初始化
InitSocialProviders(app.Env.SocialBaseURL)
```

## 5. 数据库结构

用户体系已经有用户表，增加一张“第三方账号绑定表”。

新增表字段：

```sql
CREATE TABLE social_accounts (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    name VARCHAR(255),
    avatar_url TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(provider, provider_subject)
);
```

## 6. Domain 层扩展

新增文件：`domain/social_login.go`

```go
package domain

type SocialProvider string

const (
	SocialGoogle SocialProvider = "google"
	SocialGitHub SocialProvider = "github"
)

type SocialAccount struct {
	ID             int64
	UserID         int64
	Provider       string
	ProviderSubject string
	Email          string
	Name           string
	AvatarURL      string
	CreatedAt      string
	UpdatedAt      string
}

type SocialLoginResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         *User  `json:"user"`
}

type SocialCallbackRequest struct {
	Provider string `json:"provider"`
}
```


## 7. Repository 层

建议新增两个接口：

- `UserRepository`：复用已有用户存储
- `SocialAccountRepository`：新增第三方账号绑定管理

示意：

```go
package repository

type SocialAccountRepository interface {
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*domain.SocialAccount, error)
	FindByUserID(ctx context.Context, userID int64) ([]*domain.SocialAccount, error)
	Create(ctx context.Context, account *domain.SocialAccount) error
	Upsert(ctx context.Context, account *domain.SocialAccount) error
}
```

- 继续使用当前 `UserRepository`
- 新增一个 `social_account` 的 Repository 实现
- 只做“查/建/绑定”三件事

## 8. Usecase 层

新增文件：`usecase/social_login_usecase.go`

```go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"shadmin/domain"
	"shadmin/internal/tokenservice"
	"shadmin/repository"

	"github.com/markbates/goth"
)

type SocialLoginUsecase struct {
	UserRepo           repository.UserRepository
	SocialAccountRepo  repository.SocialAccountRepository
	TokenService       *tokenservice.TokenService
}

func NewSocialLoginUsecase(
	userRepo repository.UserRepository,
	socialAccountRepo repository.SocialAccountRepository,
	tokenService *tokenservice.TokenService,
) *SocialLoginUsecase {
	return &SocialLoginUsecase{
		UserRepo:          userRepo,
		SocialAccountRepo: socialAccountRepo,
		TokenService:      tokenService,
	}
}

func (u *SocialLoginUsecase) HandleCallback(ctx context.Context, provider string, profile goth.User) (*domain.SocialLoginResult, error) {
	// 1. 先查第三方账号是否已绑定
	account, err := u.SocialAccountRepo.FindByProviderAndSubject(ctx, provider, profile.UserID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// 2. 如果没有绑定，尝试根据邮箱查用户
	var user *domain.User
	if account != nil {
		user, err = u.UserRepo.GetByID(ctx, account.UserID)
		if err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(profile.Email) != "" {
		user, err = u.UserRepo.GetByUserName(ctx, profile.Email)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}

	// 3. 如果用户不存在，创建新用户
	if user == nil {
		user, err = u.createUserFromSocial(profile)
		if err != nil {
			return nil, err
		}
	}

	// 4. 绑定第三方账号
	err = u.SocialAccountRepo.Upsert(ctx, &domain.SocialAccount{
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: profile.UserID,
		Email:           profile.Email,
		Name:            profile.Name,
		AvatarURL:       profile.AvatarURL,
	})
	if err != nil {
		return nil, err
	}

	// 5. 复用现有的 JWT 生成逻辑
	tokenPair, err := u.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &domain.SocialLoginResult{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         user,
	}, nil
}

func (u *SocialLoginUsecase) createUserFromSocial(profile goth.User) (*domain.User, error) {
	// 例如：用户名 = email，密码随机生成，默认角色 = user
	// 注意：不要把密码设为空，至少要能安全创建
	return nil, fmt.Errorf("TODO: implement create user from social profile")
}

func (u *SocialLoginUsecase) issueTokenPair(ctx context.Context, user *domain.User) (*domain.TokenPair, error) {
	// 复用现有 token service
	// 示例：
	// return u.TokenService.GeneratePair(ctx, user)
	return nil, fmt.Errorf("TODO: reuse existing token generation")
}
```

> 重点：
> - 这里最关键的是“复用你已有登录时的 token 生成逻辑”
> - 不要让第三方登录走一套独立 token 流程


## 9. Controller 层

新增文件：`api/controller/social_auth_controller.go`

```go
package controller

import (
	"net/http"

	"shadmin/domain"
	"shadmin/usecase"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
)

type SocialAuthController struct {
	SocialLoginUsecase *usecase.SocialLoginUsecase
}

func (c *SocialAuthController) BeginLogin(ctx *gin.Context) {
	provider := ctx.Param("provider")

	// 推荐使用 gothic 处理 state/session
	// 这里是草图，实际可根据情况做适配
	ctx.Set("provider", provider)

	// 触发 provider 跳转
	// 这里假设你在路由层把 provider 的值传进去
	_ = provider

	gothic.BeginAuthHandler(ctx.Writer, ctx.Request)
}

func (c *SocialAuthController) Callback(ctx *gin.Context) {
	provider := ctx.Param("provider")

	user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.RespError("social auth failed: "+err.Error()))
		return
	}

	result, err := c.SocialLoginUsecase.HandleCallback(ctx, provider, user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, domain.RespSuccess(result))
}
```

> 如果评估后觉得 `gothic` 的 session 方式和你现有系统冲突，也可以改成：
> - 走现有的 state/session cookie
> - 由后端在 `BeginLogin` 生成一个临时 state
> - 在 callback 中校验 state
> - 然后继续执行后续逻辑

---

## 10. 路由设计

建议新增一组公共路由（不需要登录）：

```go
// api/route/social.go
package route

import (
	"shadmin/api/controller"
	"shadmin/bootstrap"

	"github.com/gin-gonic/gin"
)

func RegisterSocialRoutes(apiV1 *gin.RouterGroup, app *bootstrap.Application, factory *ControllerFactory) {
	socialController := controller.SocialAuthController{
		SocialLoginUsecase: factory.CreateSocialLoginUsecase(),
	}

	authGroup := apiV1.Group("/auth/social")
	{
		authGroup.GET("/:provider", socialController.BeginLogin)
		authGroup.GET("/:provider/callback", socialController.Callback)
	}
}
```

然后在 `api/route/route.go` 里注册：

```go
func setupApiRoutes(app *bootstrap.Application, timeout time.Duration, engine *gin.Engine) {
	apiV1 := engine.Group(ApiUri)
	factory := NewControllerFactory(app, timeout, app.DB)

	publicRoutes := NewPublicRoutes(factory)
	publicRoutes.Setup(apiV1, app)

	// 新增
	RegisterSocialRoutes(apiV1, app, factory)

	protectedRoutes := NewProtectedRoutes(factory)
	protectedRoutes.Setup(apiV1, app, engine)
}
```

---

## 11. 前端接入

### 11.1 登录页增加按钮

在 `frontend/src/features/auth/sign-in` 里加几个按钮：

```tsx
<a
  href="/api/v1/auth/social/google"
  className="inline-flex items-center justify-center rounded-md border px-4 py-2"
>
  Continue with Google
</a>

<a
  href="/api/v1/auth/social/github"
  className="inline-flex items-center justify-center rounded-md border px-4 py-2"
>
  Continue with GitHub
</a>

```

### 11.2 处理回调

推荐做法是：

- 后端登录成功后，重定向到前端页面，例如：
  - `/oauth/callback?accessToken=...&refreshToken=...`

前端在这个页面里做：

```tsx
import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

export default function OAuthCallbackPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()

  useEffect(() => {
    const accessToken = params.get('accessToken')
    const refreshToken = params.get('refreshToken')

    if (accessToken && refreshToken) {
      localStorage.setItem('accessToken', accessToken)
      localStorage.setItem('refreshToken', refreshToken)
      navigate('/system/dashboard')
    } else {
      navigate('/sign-in')
    }
  }, [params, navigate])

  return <div>Logging you in...</div>
}
```

---

## 12. 推荐后端回调跳转方式

为了让前端更顺滑，使用重定向而不是纯 JSON：

### 逻辑流程

1. 用户访问 `/api/v1/auth/social/google`
2. 后端重定向到 Google 登录页
3. Google 回调 `/api/v1/auth/social/google/callback`
4. 后端成功拿到 profile 后：
   - 生成 shadmin JWT
   - 重定向到前端：
     `http://localhost:5173/oauth/callback?accessToken=...&refreshToken=...`

实现骨架：

```go
func (c *SocialAuthController) Callback(ctx *gin.Context) {
	provider := ctx.Param("provider")

	user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.Redirect(http.StatusTemporaryRedirect, "/sign-in?error=oauth")
		return
	}

	result, err := c.SocialLoginUsecase.HandleCallback(ctx, provider, user)
	if err != nil {
		ctx.Redirect(http.StatusTemporaryRedirect, "/sign-in?error=oauth")
		return
	}

	redirectURL := fmt.Sprintf(
		"%s/oauth/callback?accessToken=%s&refreshToken=%s",
		os.Getenv("SOCIAL_REDIRECT_URL"),
		url.QueryEscape(result.AccessToken),
		url.QueryEscape(result.RefreshToken),
	)

	ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
```
