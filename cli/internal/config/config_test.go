package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	t.Setenv("SHADMIN_CONFIG", p)
	t.Setenv("SHADMIN_SERVER", "")

	in := &Config{
		ServerURL:    "http://localhost:55667",
		AccessToken:  "a.b.c",
		RefreshToken: "r.r.r",
	}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 文件权限 0600
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
	if got.ServerURL != in.ServerURL || got.AccessToken != in.AccessToken || got.RefreshToken != in.RefreshToken {
		t.Errorf("roundtrip mismatch: %+v vs %+v", got, in)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "")

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
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "http://env-host:1234")

	_ = Save(&Config{ServerURL: "http://file-host", AccessToken: "file-token"})
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL != "http://env-host:1234" {
		t.Errorf("env SHADMIN_SERVER not applied: %q", c.ServerURL)
	}
	if c.AccessToken != "file-token" {
		t.Errorf("token should come from config file, got %q", c.AccessToken)
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "")

	_ = Save(&Config{ServerURL: "http://x", AccessToken: "a", RefreshToken: "r"})
	if err := Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	c, _ := Load()
	if c.AccessToken != "" || c.RefreshToken != "" {
		t.Errorf("clear should wipe creds, got %+v", c)
	}
	if c.ServerURL != "http://x" {
		t.Errorf("clear should keep SHADMIN_SERVER, got %q", c.ServerURL)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "SHADMIN_TOKEN=") || strings.Contains(content, "SHADMIN_REFRESH_TOKEN=") {
		t.Errorf("clear should remove token keys from config file, got:\n%s", content)
	}
}

func TestSaveDoesNotPersistServerEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "")

	// 先写入磁盘上的基线值
	if err := Save(&Config{ServerURL: "http://disk-host", AccessToken: "disk-token", RefreshToken: "disk-refresh"}); err != nil {
		t.Fatalf("baseline save: %v", err)
	}

	// 启用 server env 覆盖，模拟 refresh 流程更新 refresh token 后保存
	t.Setenv("SHADMIN_SERVER", "http://env-host")
	c, err := Load()
	if err != nil {
		t.Fatalf("load with server env: %v", err)
	}
	if c.AccessToken != "disk-token" || c.ServerURL != "http://env-host" || c.RefreshToken != "disk-refresh" {
		t.Fatalf("env not applied: %+v", c)
	}
	c.RefreshToken = "new-refresh"
	if err := Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 清除 env，再次读取磁盘，验证 env 值未被持久化
	t.Setenv("SHADMIN_SERVER", "")
	disk, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if disk.ServerURL != "http://disk-host" {
		t.Errorf("SHADMIN_SERVER should not be overwritten by env, got %q", disk.ServerURL)
	}
	if disk.RefreshToken != "new-refresh" {
		t.Errorf("refresh token should be updated, got %q", disk.RefreshToken)
	}
}

func TestApplyServerOverrideOneShotDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "")

	if err := Save(&Config{ServerURL: "http://disk-host", AccessToken: "disk-token", RefreshToken: "disk-refresh"}); err != nil {
		t.Fatalf("baseline save: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ApplyServerOverride(c, "http://flag-host", false)
	if c.ServerURL != "http://flag-host" {
		t.Fatalf("flag override not applied: %q", c.ServerURL)
	}
	c.RefreshToken = "new-refresh"
	if err := Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	disk, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if disk.ServerURL != "http://disk-host" {
		t.Errorf("one-shot --server should not persist, got %q", disk.ServerURL)
	}
	if disk.RefreshToken != "new-refresh" {
		t.Errorf("other fields should still persist, got refresh token %q", disk.RefreshToken)
	}
}

func TestApplyServerOverridePersist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHADMIN_CONFIG", filepath.Join(dir, ".env"))
	t.Setenv("SHADMIN_SERVER", "http://env-host")

	if err := Save(&Config{ServerURL: "http://disk-host", AccessToken: "disk-token"}); err != nil {
		t.Fatalf("baseline save: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL != "http://env-host" {
		t.Fatalf("env override not applied: %q", c.ServerURL)
	}
	ApplyServerOverride(c, "http://login-host", true)
	if err := Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Setenv("SHADMIN_SERVER", "")
	disk, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if disk.ServerURL != "http://login-host" {
		t.Errorf("login --server should persist, got %q", disk.ServerURL)
	}
}

func TestLoadParsesDotEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	t.Setenv("SHADMIN_CONFIG", p)
	t.Setenv("SHADMIN_SERVER", "")

	if err := os.WriteFile(p, []byte("SHADMIN_SERVER=http://localhost:55667\nSHADMIN_TOKEN=a.b.c\nSHADMIN_REFRESH_TOKEN=r.r.r\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL != "http://localhost:55667" || c.AccessToken != "a.b.c" || c.RefreshToken != "r.r.r" {
		t.Fatalf("parsed config mismatch: %+v", c)
	}
}
