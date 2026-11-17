package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type UserIdentityController struct {
	UserIdentityUsecase domain.UserIdentityUsecase
	RedirectURL         string // 登录成功后将短期 code 重定向的前端地址
}

// gothic 不读取 Gin 路径参数，需通过 context 传入 provider。
func injectProvider(c *gin.Context) {
	provider := c.Param("provider")
	req := c.Request.WithContext(context.WithValue(c.Request.Context(), gothic.ProviderParamKey, provider))
	c.Request = req
}

// BeginLogin godoc
//
//	@Summary		Begin identity login
//	@Description	Redirect to the OAuth provider authorization page
//	@Tags			Authentication
//	@Produce		json
//	@Param			provider	path	string	true	"OAuth provider (google / github)"
//	@Router			/auth/identity/{provider} [get]
func (sc *UserIdentityController) BeginLogin(c *gin.Context) {
	if _, err := goth.GetProvider(c.Param("provider")); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(domain.ErrUserIdentityProviderDisabled.Error()))
		return
	}
	injectProvider(c)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Callback godoc
//
//	@Summary		User identity login callback
//	@Description	OAuth provider redirects here; exchanges code for profile and issues JWT
//	@Tags			Authentication
//	@Produce		json
//	@Param			provider	path	string	true	"OAuth provider (google / github)"
//	@Router			/auth/identity/{provider}/callback [get]
func (sc *UserIdentityController) Callback(c *gin.Context) {
	injectProvider(c)

	provider := c.Param("provider")
	meta := domain.LoginMeta{
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.Header.Get("User-Agent"),
	}

	oidcUser, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		sc.UserIdentityUsecase.RecordFailure(c.Request.Context(), provider, meta, "第三方身份认证失败")
		sc.redirectError(c)
		return
	}

	code, err := sc.UserIdentityUsecase.HandleCallback(c.Request.Context(), provider, toIdentityProfile(oidcUser), meta)
	if err != nil {
		if errors.Is(err, domain.ErrUserDisabled) {
			sc.redirectTo(c, sc.errorRedirectURL("disabled"))
			return
		}
		sc.redirectError(c)
		return
	}

	target, err := url.Parse(sc.RedirectURL)
	if err != nil || target.String() == "" {
		c.JSON(http.StatusInternalServerError, domain.RespError("identity redirect not configured"))
		return
	}
	// 通过一次性 code 换取令牌，避免 JWT 暴露在 URL 中。
	q := target.Query()
	q.Set("code", code)
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}

func toIdentityProfile(u goth.User) domain.UserIdentityProfile {
	return domain.UserIdentityProfile{
		UserID:    u.UserID,
		Email:     u.Email,
		Name:      u.Name,
		NickName:  u.NickName,
		AvatarURL: u.AvatarURL,
	}
}

// Exchange godoc
//
//	@Summary		Exchange identity-login callback code for JWT tokens
//	@Description	Exchanges a one-time code issued by the OAuth callback for access/refresh tokens.
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.UserIdentityExchangeRequest				true	"Identity login callback code"
//	@Success		200		{object}	domain.Response{data=domain.UserIdentityResult}	"Tokens exchanged successfully"
//	@Failure		400		{object}	domain.Response									"Invalid request"
//	@Failure		404		{object}	domain.Response									"Code expired or invalid"
//	@Router			/auth/identity/exchange [post]
func (sc *UserIdentityController) Exchange(c *gin.Context) {
	var req domain.UserIdentityExchangeRequest
	if !MustBindJSON(c, &req) {
		return
	}

	result, err := sc.UserIdentityUsecase.Exchange(c.Request.Context(), req.Code)
	if err != nil {
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// ListProviders godoc
//
//	@Summary		List enabled identity providers
//	@Description	Returns the list of OAuth providers enabled by the backend
//	@Tags			Authentication
//	@Produce		json
//	@Success		200	{object}	domain.Response{data=[]string}
//	@Router			/auth/identity/providers [get]
func (sc *UserIdentityController) ListProviders(c *gin.Context) {
	providers := goth.GetProviders()
	list := make([]string, 0, len(providers))
	for _, p := range providers {
		list = append(list, p.Name())
	}
	c.JSON(http.StatusOK, domain.RespSuccess(list))
}

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

// errorRedirectURL 使用前端回调地址同源的 /sign-in 页面。
func (sc *UserIdentityController) errorRedirectURL(code string) string {
	u, err := url.Parse(sc.RedirectURL)
	if err != nil || u.Host == "" {
		return "/sign-in?error=" + code
	}
	return u.Scheme + "://" + u.Host + "/sign-in?error=" + code
}
