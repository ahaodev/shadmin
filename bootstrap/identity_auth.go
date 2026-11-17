package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"shadmin/internal/conf"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
)

// InitIdentityProviders 注册已配置的 provider，并在 OAuth 流程开始前初始化 state cookie store。
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
		slog.Info("Identity login: no provider configured (skipped)")
		return
	}
	slog.Info("Identity login: providers enabled", "providers", strings.Join(registered, ", "))
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

func callbackURL(baseURL, provider string) string {
	return fmt.Sprintf("%s/api/v1/auth/identity/%s/callback", strings.TrimRight(baseURL, "/"), provider)
}
