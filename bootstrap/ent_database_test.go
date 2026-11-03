package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"shadmin/ent"
	"shadmin/internal/conf"
)

func TestProductionStartupCreatesSchema(t *testing.T) {
	t.Chdir(t.TempDir())
	client, err := NewEntDatabase(&conf.Env{AppEnv: conf.AppEnvProd, DBType: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.User.Query().Count(context.Background()); err != nil {
		t.Fatal(err)
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
