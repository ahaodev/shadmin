package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHADMIN_CONFIG", "")
	t.Setenv("SHADMIN_SERVER", "")
	t.Setenv("SHADMIN_TOKEN", "")

	in := &Config{
		ServerURL:    "http://localhost:55667",
		Username:     "alice",
		AccessToken:  "a.b.c",
		RefreshToken: "r.r.r",
	}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 文件权限 0600
	p := filepath.Join(dir, "shadmin", "config.yaml")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected 0600, got %o", perm)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ServerURL != in.ServerURL || got.Username != in.Username ||
		got.AccessToken != in.AccessToken || got.RefreshToken != in.RefreshToken {
		t.Errorf("roundtrip mismatch: %+v vs %+v", got, in)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHADMIN_CONFIG", "")
	t.Setenv("SHADMIN_SERVER", "")
	t.Setenv("SHADMIN_TOKEN", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL != "" || c.AccessToken != "" {
		t.Errorf("expected empty config, got %+v", c)
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHADMIN_CONFIG", "")
	t.Setenv("SHADMIN_SERVER", "http://env-host:1234")
	t.Setenv("SHADMIN_TOKEN", "env-token")

	_ = Save(&Config{ServerURL: "http://file-host", AccessToken: "file-token"})
	c, err := LoadWithEnv()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL != "http://env-host:1234" {
		t.Errorf("env SHADMIN_SERVER not applied: %q", c.ServerURL)
	}
	if c.AccessToken != "env-token" {
		t.Errorf("env SHADMIN_TOKEN not applied: %q", c.AccessToken)
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHADMIN_CONFIG", "")
	t.Setenv("SHADMIN_SERVER", "")
	t.Setenv("SHADMIN_TOKEN", "")

	_ = Save(&Config{ServerURL: "http://x", Username: "u", AccessToken: "a", RefreshToken: "r"})
	if err := Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	c, _ := Load()
	if c.AccessToken != "" || c.RefreshToken != "" || c.Username != "" {
		t.Errorf("clear should wipe creds, got %+v", c)
	}
	if c.ServerURL != "http://x" {
		t.Errorf("clear should keep server_url, got %q", c.ServerURL)
	}
}

func TestSaveDoesNotPersistEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("SHADMIN_CONFIG", "")
	t.Setenv("SHADMIN_SERVER", "")
	t.Setenv("SHADMIN_TOKEN", "")

	// 先写入磁盘上的基线值
	if err := Save(&Config{ServerURL: "http://disk-host", AccessToken: "disk-token", RefreshToken: "disk-refresh"}); err != nil {
		t.Fatalf("baseline save: %v", err)
	}

	// 启用 env 覆盖，模拟 refresh 流程更新 refresh_token 后保存
	t.Setenv("SHADMIN_SERVER", "http://env-host")
	t.Setenv("SHADMIN_TOKEN", "env-token")
	c, err := LoadWithEnv()
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}
	if c.AccessToken != "env-token" || c.ServerURL != "http://env-host" {
		t.Fatalf("env not applied: %+v", c)
	}
	c.RefreshToken = "new-refresh"
	if err := Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 清除 env，再次读取磁盘，验证 env 值未被持久化
	t.Setenv("SHADMIN_SERVER", "")
	t.Setenv("SHADMIN_TOKEN", "")
	disk, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if disk.AccessToken != "disk-token" {
		t.Errorf("access_token should not be overwritten by env, got %q", disk.AccessToken)
	}
	if disk.ServerURL != "http://disk-host" {
		t.Errorf("server_url should not be overwritten by env, got %q", disk.ServerURL)
	}
	if disk.RefreshToken != "new-refresh" {
		t.Errorf("refresh_token should be updated, got %q", disk.RefreshToken)
	}
}
