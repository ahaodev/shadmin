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

// SocialAuthController 处理第三方登录入口回调：
// - 入口路径 GET /auth/social/:provider        → 重定向到 provider 授权页
// - 回调路径 GET /auth/social/:provider/callback → 拿到 profile 后签发 JWT，重定向回前端
// - 列表接口 GET /auth/social/providers         → 返回当前已启用的 provider
type SocialAuthController struct {
	SocialLoginUsecase domain.SocialLoginUsecase
	RedirectURL        string // 登录成功后将短期 code 重定向的前端地址
	codeStore          *socialCodeStore
}

type socialCodeStore struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]socialCodeEntry
}

type socialCodeEntry struct {
	result    *domain.SocialLoginResult
	expiresAt time.Time
}

func newSocialCodeStore(ttl time.Duration) *socialCodeStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &socialCodeStore{
		ttl:     ttl,
		entries: make(map[string]socialCodeEntry),
	}
}

func (s *socialCodeStore) Put(result *domain.SocialLoginResult) (string, error) {
	if result == nil {
		return "", errors.New("social login result is nil")
	}

	code, err := randomCode(24)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[code] = socialCodeEntry{
		result:    result,
		expiresAt: time.Now().Add(s.ttl),
	}
	return code, nil
}

func (s *socialCodeStore) Consume(code string) *domain.SocialLoginResult {
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

func (sc *SocialAuthController) getCodeStore() *socialCodeStore {
	if sc.codeStore == nil {
		sc.codeStore = newSocialCodeStore(5 * time.Minute)
	}
	return sc.codeStore
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
// @Summary      Begin social login
// @Description  Redirect to the OAuth provider authorization page
// @Tags         Authentication
// @Produce      json
// @Param        provider  path  string  true  "OAuth provider (google / github)"
// @Router       /auth/social/{provider} [get]
func (sc *SocialAuthController) BeginLogin(c *gin.Context) {
	if _, err := goth.GetProvider(c.Param("provider")); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(domain.ErrSocialProviderDisabled.Error()))
		return
	}
	injectProvider(c)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Callback godoc
// @Summary      Social login callback
// @Description  OAuth provider redirects here; exchanges code for profile and issues JWT
// @Tags         Authentication
// @Produce      json
// @Param        provider  path  string  true  "OAuth provider (google / github)"
// @Router       /auth/social/{provider}/callback [get]
func (sc *SocialAuthController) Callback(c *gin.Context) {
	injectProvider(c)

	profile, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		sc.redirectError(c)
		return
	}

	result, err := sc.SocialLoginUsecase.HandleCallback(c.Request.Context(), c.Param("provider"), toSocialProfile(profile))
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
		c.JSON(http.StatusInternalServerError, domain.RespError("failed to prepare social login callback"))
		return
	}

	target, err := url.Parse(sc.RedirectURL)
	if err != nil || target.String() == "" {
		c.JSON(http.StatusInternalServerError, domain.RespError("social redirect not configured"))
		return
	}
	q := target.Query()
	q.Set("code", code)
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}

// Exchange godoc
// @Summary      Exchange social-login callback code for JWT tokens
// @Description  Exchanges a one-time code issued by the OAuth callback for access/refresh tokens.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.SocialExchangeRequest  true  "Social login callback code"
// @Success      200  {object}  domain.Response{data=domain.SocialLoginResult}  "Tokens exchanged successfully"
// @Failure      400  {object}  domain.Response  "Invalid request"
// @Failure      404  {object}  domain.Response  "Code expired or invalid"
// @Router       /auth/social/exchange [post]
func (sc *SocialAuthController) Exchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("invalid request"))
		return
	}

	codeStore := sc.getCodeStore()
	result := codeStore.Consume(req.Code)
	if result == nil {
		c.JSON(http.StatusNotFound, domain.RespError("social login code expired or invalid"))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// ListProviders godoc
// @Summary      List enabled social providers
// @Description  Returns the list of OAuth providers enabled by the backend
// @Tags         Authentication
// @Produce      json
// @Success      200  {object}  domain.Response{data=[]domain.SocialProviderInfo}
// @Router       /auth/social/providers [get]
func (sc *SocialAuthController) ListProviders(c *gin.Context) {
	providers := goth.GetProviders()
	list := make([]domain.SocialProviderInfo, 0, len(providers))
	for _, p := range providers {
		list = append(list, domain.SocialProviderInfo{
			Provider: p.Name(),
			Name:     providerDisplayName(p.Name()),
		})
	}
	c.JSON(http.StatusOK, domain.RespSuccess(list))
}

// redirectTo 重定向到给定 URL；若 URL 解析失败则降级返回 JSON 错误。
func (sc *SocialAuthController) redirectTo(c *gin.Context, target string) {
	if target == "" {
		c.JSON(http.StatusInternalServerError, domain.RespError("social redirect not configured"))
		return
	}
	c.Redirect(http.StatusFound, target)
}

func (sc *SocialAuthController) redirectError(c *gin.Context) {
	sc.redirectTo(c, sc.errorRedirectURL("oauth"))
}

// errorRedirectURL 生成前端错误重定向地址：<SocialRedirectURL 的同源 /sign-in>?error=<code>
// 这里假设前端 SOCIAL_REDIRECT_URL 形如 http://host/oauth-callback，
// 同宿主的 /sign-in 即为登录失败落地页。
func (sc *SocialAuthController) errorRedirectURL(code string) string {
	u, err := url.Parse(sc.RedirectURL)
	if err != nil || u.Host == "" {
		return "/sign-in?error=" + code
	}
	return u.Scheme + "://" + u.Host + "/sign-in?error=" + code
}

func toSocialProfile(p goth.User) *domain.SocialProfile {
	return &domain.SocialProfile{
		UserID:    p.UserID,
		Email:     p.Email,
		Name:      p.Name,
		NickName:  p.NickName,
		AvatarURL: p.AvatarURL,
	}
}

func providerDisplayName(name string) string {
	switch name {
	case domain.SocialGoogle:
		return "Google"
	case domain.SocialGitHub:
		return "GitHub"
	default:
		if name == "" {
			return ""
		}
		return name[:1] + name[1:]
	}
}
