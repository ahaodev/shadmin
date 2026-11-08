package conf

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestProductionSecrets(t *testing.T) {
	base := Env{
		AppEnv: AppEnvProd, ContextTimeout: 60, AuthzSyncPollIntervalSeconds: 3600,
		Port: ":55667", DBType: DbTypeSqlite, CacheType: CacheTypeMem,
		StorageType: StoreTypeDisk, StorageBasePath: ".uploads",
		AccessTokenExpiryMinute: 180, RefreshTokenExpiryMinute: 1800,
		AccessTokenSecret: "configured-access-secret-for-test", RefreshTokenSecret: "configured-refresh-secret-for-test",
		IdentitySessionSecret: defaultIdentitySessionSecret, AdminUsername: "admin", AdminPassword: "123",
	}
	for _, test := range []struct {
		name   string
		change func(*Env)
		want   string
	}{
		{"configured JWT keys without OAuth", func(*Env) {}, ""},
		{"access default", func(e *Env) { e.AccessTokenSecret = defaultAccessTokenSecret }, "ACCESS_TOKEN_SECRET"},
		{"refresh default", func(e *Env) { e.RefreshTokenSecret = defaultRefreshTokenSecret }, "REFRESH_TOKEN_SECRET"},
		{"swapped public default", func(e *Env) { e.AccessTokenSecret = defaultRefreshTokenSecret }, "ACCESS_TOKEN_SECRET"},
		{"missing key", func(e *Env) { e.AccessTokenSecret = "" }, "ACCESS_TOKEN_SECRET"},
		{"Google default session", func(e *Env) { e.GoogleClientID, e.GoogleClientSecret = "id", "secret" }, "IDENTITY_SESSION_SECRET"},
		{"GitHub default session", func(e *Env) { e.GitHubClientID, e.GitHubClientSecret = "id", "secret" }, "IDENTITY_SESSION_SECRET"},
		{"configured OAuth session", func(e *Env) {
			e.GoogleClientID, e.GoogleClientSecret = "id", "secret"
			e.IdentitySessionSecret = "configured-session-secret-for-test"
		}, ""},
		{"dev defaults", func(e *Env) {
			e.AppEnv = AppEnvDev
			e.AccessTokenSecret, e.RefreshTokenSecret = defaultAccessTokenSecret, defaultRefreshTokenSecret
		}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := base
			test.change(&env)
			err := env.validate()
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate = %v; want error naming %s", err, test.want)
			}
		})
	}
}

func TestNewEnvRejectsProductionDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("APP_ENV", AppEnvProd)
	t.Setenv("ACCESS_TOKEN_SECRET", defaultAccessTokenSecret)
	t.Setenv("REFRESH_TOKEN_SECRET", defaultRefreshTokenSecret)
	_, err := NewEnv()
	if err == nil || !strings.Contains(err.Error(), "ACCESS_TOKEN_SECRET") || !strings.Contains(err.Error(), "REFRESH_TOKEN_SECRET") {
		t.Fatalf("NewEnv = %v; want production default key errors", err)
	}
}
