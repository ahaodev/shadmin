package bootstrap

import (
	"bytes"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"shadmin/ent"
	"shadmin/internal/conf"
)

func TestProductionRequiresExplicitMigration(t *testing.T) {
	t.Chdir(t.TempDir())
	env := &conf.Env{AppEnv: conf.AppEnvProd, DBType: "sqlite", DBAutoMigrate: true}
	if client, err := NewEntDatabase(env); err == nil {
		client.Close()
		t.Fatal("production created schema implicitly")
	}
	if err := MigrateDatabase(env); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "./.database/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("ALTER TABLE users ADD COLUMN legacy_data TEXT; INSERT INTO users (id, username, created_at, updated_at, legacy_data) VALUES ('legacy', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'keep')"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{conf.AppEnvDev, conf.AppEnvProd} {
		env.AppEnv = mode
		client, err := NewEntDatabase(env)
		if err != nil {
			t.Fatal(err)
		}
		client.Close()
		var value string
		if err := db.QueryRow("SELECT legacy_data FROM users WHERE id='legacy'").Scan(&value); err != nil || value != "keep" {
			t.Fatalf("legacy data = %q, %v", value, err)
		}
	}
}

func TestDevelopmentAutoMigrate(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			t.Chdir(t.TempDir())
			env := &conf.Env{AppEnv: conf.AppEnvDev, DBType: "sqlite", DBAutoMigrate: enabled}
			client, err := NewEntDatabase(env)
			if enabled {
				if err != nil {
					t.Fatal(err)
				}
				client.Close()
			} else if err == nil {
				client.Close()
				t.Fatal("created schema with auto-migration disabled")
			}
		})
	}
}

func TestDatabaseConnectionLogsExcludeCredentials(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	for _, test := range []struct {
		name    string
		connect func(*conf.Env) (*ent.Client, error)
		dsn     string
	}{
		{"mysql", connectMySQL, "user:private-password@tcp(localhost:3306)/shadmin"},
		{"postgres", connectPostgreSQL, "postgres://user:private-password@localhost/shadmin"},
		{"invalid mysql", connectMySQL, "private-password"},
	} {
		client, err := test.connect(&conf.Env{DBDSN: test.dsn})
		if client != nil {
			client.Close()
		}
		if err != nil && strings.Contains(err.Error(), "private-password") {
			t.Fatalf("%s error exposed credentials", test.name)
		}
	}
	if strings.Contains(logs.String(), "private-password") {
		t.Fatal("connection logs exposed credentials")
	}
}
