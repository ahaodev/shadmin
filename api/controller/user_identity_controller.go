package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"shadmin/internal/auth"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

// UserIdentityController 处理第三方登录入口回调：
// - 入口路径 GET /auth/identity/:provider        → 重定向到 provider 授权页
// - 回调路径 GET /auth/identity/:provider/callback → 拿到 profile 后签发 JWT，重定向回前端
// - 列表接口 GET /auth/identity/providers         → 返回当前已启用的 provider
type UserIdentityController struct {
	UserIdentityUsecase domain.UserIdentityUsecase
	RedirectURL         string // 登录成功后将短期 code 重定向的前端地址
	CodeStore           *auth.UserIdentityCodeStore
}

const providerCtxKey = "provider"

// 因为 gin 的 :provider 路径参数不会进入 req.URL.Query()，而 gothic 默认从 query
// 或 ctx 中读取 provider，因此这里把 provider 注入 request context 再交给 gothic。
func injectProvider(c *gin.Context) {
	provider := c.Param("provider")
	req := c.Request.WithContext(context.WithValue(c.Request.Context(), providerCtxKey, provider))
	c.Request = req
}

// BeginLogin godoc
// @Summary      Begin identity login
// @Description  Redirect to the OAuth provider authorization page
// @Tags         Authentication
// @Produce      json
// @Param        provider  path  string  true  "OAuth provider (google / github)"
// @Router       /auth/identity/{provider} [get]
func (sc *UserIdentityController) BeginLogin(c *gin.Context) {
	if _, err := goth.GetProvider(c.Param("provider")); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(domain.ErrUserIdentityProviderDisabled.Error()))
		return
	}
	injectProvider(c)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Callback godoc
// @Summary      User identity login callback
// @Description  OAuth provider redirects here; exchanges code for profile and issues JWT
// @Tags         Authentication
// @Produce      json
// @Param        provider  path  string  true  "OAuth provider (google / github)"
// @Router       /auth/identity/{provider}/callback [get]
func (sc *UserIdentityController) Callback(c *gin.Context) {
	injectProvider(c)

	// 1) 在后端回调中完成 provider 身份校验并拿到第三方用户资料。
	profile, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		sc.redirectError(c)
		return
	}

	// 2) 绑定/创建本地用户并生成 JWT 结果（access/refresh + provider avatar）。
	result, err := sc.UserIdentityUsecase.HandleCallback(c.Request.Context(), c.Param("provider"), toUserIdentityProfile(profile))
	if err != nil {
		if errors.Is(err, domain.ErrUserDisabled) {
			sc.redirectTo(c, sc.errorRedirectURL("disabled"))
			return
		}
		sc.redirectError(c)
		return
	}

	// 3) 不把 JWT 放进 URL；改为生成一次性短码，供前端随后 POST /exchange 换取 token。
	code, err := sc.CodeStore.Put(c.Request.Context(), result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError("failed to prepare identity login callback"))
		return
	}

	target, err := url.Parse(sc.RedirectURL)
	if err != nil || target.String() == "" {
		c.JSON(http.StatusInternalServerError, domain.RespError("identity redirect not configured"))
		return
	}
	q := target.Query()
	q.Set("code", code)
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}

// Exchange godoc
// @Summary      Exchange identity-login callback code for JWT tokens
// @Description  Exchanges a one-time code issued by the OAuth callback for access/refresh tokens.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.UserIdentityExchangeRequest  true  "Identity login callback code"
// @Success      200  {object}  domain.Response{data=domain.UserIdentityResult}  "Tokens exchanged successfully"
// @Failure      400  {object}  domain.Response  "Invalid request"
// @Failure      404  {object}  domain.Response  "Code expired or invalid"
// @Router       /auth/identity/exchange [post]
func (sc *UserIdentityController) Exchange(c *gin.Context) {
	var req domain.UserIdentityExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("invalid request"))
		return
	}

	// 一次性消费短码：成功即删除，避免重放；过期/无效返回 404。
	result, err := sc.CodeStore.Consume(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError("failed to exchange identity login code"))
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, domain.RespError("identity login code expired or invalid"))
		return
	}

	// 前端在自身上下文中拿到 token 并建立登录态，避免 token 暴露在重定向 URL 中。
	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// ListProviders godoc
// @Summary      List enabled identity providers
// @Description  Returns the list of OAuth providers enabled by the backend
// @Tags         Authentication
// @Produce      json
// @Success      200  {object}  domain.Response{data=[]string}
// @Router       /auth/identity/providers [get]
func (sc *UserIdentityController) ListProviders(c *gin.Context) {
	providers := goth.GetProviders()
	list := make([]string, 0, len(providers))
	for _, p := range providers {
		list = append(list, providerDisplayName(p.Name()))
	}
	c.JSON(http.StatusOK, domain.RespSuccess(list))
}

// redirectTo 重定向到给定 URL；若 URL 解析失败则降级返回 JSON 错误。
func (sc *UserIdentityController) redirectTo(c *gin.Context, target string) {
	if target == "" {
		c.JSON(http.StatusInternalServerError, domain.RespError("identity redirect not configured"))
		return
	}
	c.Redirect(http.StatusFound, target)
}

func (sc *UserIdentityController) redirectError(c *gin.Context) {
	sc.redirectTo(c, sc.errorRedirectURL("oauth"))
}

// errorRedirectURL 生成前端错误重定向地址：<IdentityRedirectURL 的同源 /sign-in>?error=<code>
// 这里假设前端 IDENTITY_REDIRECT_URL 形如 http://host/oauth-callback，
// 同宿主的 /sign-in 即为登录失败落地页。
func (sc *UserIdentityController) errorRedirectURL(code string) string {
	u, err := url.Parse(sc.RedirectURL)
	if err != nil || u.Host == "" {
		return "/sign-in?error=" + code
	}
	return u.Scheme + "://" + u.Host + "/sign-in?error=" + code
}

func toUserIdentityProfile(p goth.User) *domain.UserIdentityProfile {
	return &domain.UserIdentityProfile{
		UserID:    p.UserID,
		Email:     p.Email,
		Name:      p.Name,
		NickName:  p.NickName,
		AvatarURL: p.AvatarURL,
	}
}

func providerDisplayName(name string) string {
	switch name {
	case domain.ProviderGoogle:
		return "Google"
	case domain.ProviderGithub:
		return "GitHub"
	default:
		if name == "" {
			return ""
		}
		return name[:1] + name[1:]
	}
}
