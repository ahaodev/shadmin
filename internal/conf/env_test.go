package conf

import (
	"os"
	"strings"
	"testing"
)

func TestAutoMigrateConfig(t *testing.T) {
	for _, test := range []struct {
		name, config, override string
		want                   bool
	}{
		{name: "default and generated config", want: true},
		{name: "config disables", config: "DB_AUTO_MIGRATE=false\n", want: false},
		{name: "environment disables", config: "DB_AUTO_MIGRATE=true\n", override: "false", want: false},
		{name: "environment enables", config: "DB_AUTO_MIGRATE=false\n", override: "true", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("APP_ENV", AppEnvDev)
			t.Setenv("DB_AUTO_MIGRATE", test.override)
			if test.config != "" {
				if err := os.WriteFile(".env", []byte(test.config), 0600); err != nil {
					t.Fatal(err)
				}
			}
			env, err := NewEnv()
			if err != nil {
				t.Fatal(err)
			}
			if env.DBAutoMigrate != test.want {
				t.Fatalf("DBAutoMigrate = %t, want %t", env.DBAutoMigrate, test.want)
			}
			if test.config == "" {
				content, err := os.ReadFile(".env")
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(content), "DB_AUTO_MIGRATE=true\n") {
					t.Fatal("generated config missing DB_AUTO_MIGRATE=true")
				}
			}
		})
	}
}

func TestNewEnvReadsExistingFile(t *testing.T) {
	for _, test := range []struct {
		name, config, wantError string
	}{
		{name: "partial read-only file", config: "# Custom settings\nPORT=:9000\nCUSTOM=1"},
		{name: "invalid value", config: "CONTEXT_TIMEOUT=0\n", wantError: "CONTEXT_TIMEOUT必须大于0"},
		{name: "invalid syntax", config: "invalid dotenv line\n", wantError: "读取配置文件失败"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("APP_ENV", AppEnvDev)
			t.Setenv("DB_AUTO_MIGRATE", "")
			t.Setenv("CONTEXT_TIMEOUT", "")
			t.Setenv("PORT", ":7777")
			if err := os.WriteFile(".env", []byte(test.config), 0400); err != nil {
				t.Fatal(err)
			}

			env, err := NewEnv()
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				if !env.DBAutoMigrate || env.Port != ":7777" {
					t.Fatal("NewEnv did not apply the runtime default and environment override")
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("NewEnv = %v; want error containing %q", err, test.wantError)
			}

			content, err := os.ReadFile(".env")
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != test.config {
				t.Fatal("NewEnv modified the existing .env")
			}
		})
	}
}

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
		{"production memory cache without Redis", func(*Env) {}, ""},
		{"production Redis cache", func(e *Env) {
			e.CacheType, e.RedisAddress = CacheTypeRedis, "localhost:6379"
		}, ""},
		{"Redis cache without address", func(e *Env) { e.CacheType = CacheTypeRedis }, "REDIS_ADDRESS"},
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
	t.Setenv("APP_ENV", AppEnvProd)
	t.Setenv("ACCESS_TOKEN_SECRET", defaultAccessTokenSecret)
	t.Setenv("REFRESH_TOKEN_SECRET", defaultRefreshTokenSecret)
	_, err := NewEnv()
	if err == nil || !strings.Contains(err.Error(), "ACCESS_TOKEN_SECRET") || !strings.Contains(err.Error(), "REFRESH_TOKEN_SECRET") {
		t.Fatalf("NewEnv = %v; want production default key errors", err)
	}
}

func TestS3StorageConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", AppEnvDev)
	t.Setenv("STORAGE_TYPE", StoreTypeS3)
	t.Setenv("S3_REGION", "eu-central-1")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("S3_TOKEN", "")
	if err := os.WriteFile(".env", []byte("S3_REGION=us-east-2\n"), 0600); err != nil {
		t.Fatal(err)
	}

	env, err := NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env.S3Region != "eu-central-1" {
		t.Fatalf("S3Region = %q; want environment override", env.S3Region)
	}
	t.Setenv("S3_ACCESS_KEY", "explicit-key")
	if _, err := NewEnv(); err == nil || !strings.Contains(err.Error(), "S3_SECRET_KEY") {
		t.Fatalf("NewEnv with partial credentials = %v; want S3_SECRET_KEY error", err)
	}
}
