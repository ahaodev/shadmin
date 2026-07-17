package bootstrap

import (
	"fmt"
	"net/http"
	"strings"

	"shadmin/internal/conf"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
)

// InitIdentityProviders 根据环境变量注册已配置的第三方登录 provider，
// 并初始化 gothic 用来承载 OAuth state 的 cookie store。
//
//   - 仅当对应 provider 的 ClientID/Secret 都非空时才注册，
//     未配置的 provider 不会出现在 goth.GetProviders() 中，
//     前端 /providers 接口据此动态决定是否显示对应登录按钮。
//   - gothic.Store 必须在任何 OAuth 流程开始之前初始化，
//     否则 BeginAuth 会因没有 store 而无法写入 state。
//
// baseURL 为后端外部可达地址（env.IdentityBaseURL），用于拼接 OAuth 回调 URL。
func InitIdentityProviders(env *conf.Env) {
	gothic.Store = initGothicStore(env.IdentitySessionSecret)

	registered := make([]string, 0)
	if registerGoogleProvider(env) {
		registered = append(registered, "google")
	}
	if registerGitHubProvider(env) {
		registered = append(registered, "github")
	}

	if len(registered) == 0 {
		log.Printf("Identity login: no provider configured (skipped)")
		return
	}
	log.Printf("Identity login: providers enabled = %s", strings.Join(registered, ", "))
}

func initGothicStore(secret string) *sessions.CookieStore {
	if len(secret) == 0 {
		secret = "shadmin-identity-default-session-secret"
	}

	store := sessions.NewCookieStore([]byte(secret))
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteLaxMode
	store.Options.Path = "/"
	return store
}

func registerGoogleProvider(env *conf.Env) bool {
	if env.GoogleClientID == "" || env.GoogleClientSecret == "" {
		return false
	}

	goth.UseProviders(
		google.New(env.GoogleClientID, env.GoogleClientSecret, callbackURL(env.IdentityBaseURL, "google"), "profile", "email"),
	)
	return true
}

func registerGitHubProvider(env *conf.Env) bool {
	if env.GitHubClientID == "" || env.GitHubClientSecret == "" {
		return false
	}

	goth.UseProviders(
		github.New(env.GitHubClientID, env.GitHubClientSecret, callbackURL(env.IdentityBaseURL, "github"), "user:email"),
	)
	return true
}

// callbackURL 拼接某 provider 的 OAuth 回调地址：<baseURL>/api/v1/auth/identity/<provider>/callback
func callbackURL(baseURL, provider string) string {
	return fmt.Sprintf("%s/api/v1/auth/identity/%s/callback", strings.TrimRight(baseURL, "/"), provider)
}
