package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

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
	CodeStore           *UserIdentityStore
}

// UserIdentityStore 存储短期 OAuth 回调 code，线程安全。
type UserIdentityStore struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]userIdentityCodeEntry
}

type userIdentityCodeEntry struct {
	result    *domain.UserIdentityResult
	expiresAt time.Time
}

// NewUserIdentityStore 创建一个新的 code store，ttl 为 code 有效期。
func NewUserIdentityStore(ttl time.Duration) *UserIdentityStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &UserIdentityStore{
		ttl:     ttl,
		entries: make(map[string]userIdentityCodeEntry),
	}
}

func (s *UserIdentityStore) Put(result *domain.UserIdentityResult) (string, error) {
	if result == nil {
		return "", errors.New("identity login result is nil")
	}

	code, err := randomCode(24)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[code] = userIdentityCodeEntry{
		result:    result,
		expiresAt: time.Now().Add(s.ttl),
	}
	return code, nil
}

func (s *UserIdentityStore) Consume(code string) *domain.UserIdentityResult {
	if code == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[code]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(s.entries, code)
		return nil
	}
	delete(s.entries, code)
	return entry.result
}

func (sc *UserIdentityController) getCodeStore() *UserIdentityStore {
	if sc.CodeStore == nil {
		sc.CodeStore = NewUserIdentityStore(5 * time.Minute)
	}
	return sc.CodeStore
}

func randomCode(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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

	profile, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		sc.redirectError(c)
		return
	}

	result, err := sc.UserIdentityUsecase.HandleCallback(c.Request.Context(), c.Param("provider"), toUserIdentityProfile(profile))
	if err != nil {
		if errors.Is(err, domain.ErrUserDisabled) {
			sc.redirectTo(c, sc.errorRedirectURL("disabled"))
			return
		}
		sc.redirectError(c)
		return
	}

	codeStore := sc.getCodeStore()
	code, err := codeStore.Put(result)
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

	codeStore := sc.getCodeStore()
	result := codeStore.Consume(req.Code)
	if result == nil {
		c.JSON(http.StatusNotFound, domain.RespError("identity login code expired or invalid"))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// ListProviders godoc
// @Summary      List enabled identity providers
// @Description  Returns the list of OAuth providers enabled by the backend
// @Tags         Authentication
// @Produce      json
// @Success      200  {object}  domain.Response{data=[]domain.UserIdentityProviderInfo}
// @Router       /auth/identity/providers [get]
func (sc *UserIdentityController) ListProviders(c *gin.Context) {
	providers := goth.GetProviders()
	list := make([]domain.UserIdentityProviderInfo, 0, len(providers))
	for _, p := range providers {
		list = append(list, domain.UserIdentityProviderInfo{
			Provider: p.Name(),
			Name:     providerDisplayName(p.Name()),
		})
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
