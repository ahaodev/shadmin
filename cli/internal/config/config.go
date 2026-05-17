package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 本地 CLI 配置
type Config struct {
	ServerURL    string `yaml:"server_url"`
	Username     string `yaml:"username,omitempty"`
	AccessToken  string `yaml:"access_token,omitempty"`
	RefreshToken string `yaml:"refresh_token,omitempty"`

	// envOverridden 记录哪些字段由环境变量提供，Save 时会跳过持久化以避免把
	// 临时 SHADMIN_TOKEN/SHADMIN_SERVER 写入磁盘文件。
	envOverridden map[string]bool `yaml:"-"`
}

// Path 返回配置文件路径
// 优先使用 $XDG_CONFIG_HOME/shadmin/config.yaml，回退 ~/.shadmin/config.yaml
func Path() (string, error) {
	if p := os.Getenv("SHADMIN_CONFIG"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "shadmin", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".shadmin", "config.yaml"), nil
}

// Load 读取配置文件；文件不存在时返回空 Config（不报错）
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	applyEnvOverrides(&c)
	return &c, nil
}

// LoadWithEnv 与 Load 相同，但允许无配置文件时从环境变量构造
func LoadWithEnv() (*Config, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	applyEnvOverrides(c)
	return c, nil
}

func applyEnvOverrides(c *Config) {
	if c.envOverridden == nil {
		c.envOverridden = map[string]bool{}
	}
	if v := os.Getenv("SHADMIN_SERVER"); v != "" {
		c.ServerURL = v
		c.envOverridden["server_url"] = true
	}
	if v := os.Getenv("SHADMIN_TOKEN"); v != "" {
		c.AccessToken = v
		c.envOverridden["access_token"] = true
	}
}

// Save 以 0600 权限写回配置文件
// 由环境变量提供的字段不会被持久化到磁盘，避免把临时凭据写入用户配置。
func Save(c *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}

	toWrite := *c
	toWrite.envOverridden = nil
	if len(c.envOverridden) > 0 {
		// 读取磁盘上已有的值，恢复 env 覆盖前的字段，避免把环境变量写入磁盘
		if data, rerr := os.ReadFile(p); rerr == nil {
			var disk Config
			if uerr := yaml.Unmarshal(data, &disk); uerr == nil {
				if c.envOverridden["server_url"] {
					toWrite.ServerURL = disk.ServerURL
				}
				if c.envOverridden["access_token"] {
					toWrite.AccessToken = disk.AccessToken
				}
			}
		} else if errors.Is(rerr, os.ErrNotExist) {
			if c.envOverridden["server_url"] {
				toWrite.ServerURL = ""
			}
			if c.envOverridden["access_token"] {
				toWrite.AccessToken = ""
			}
		}
	}

	data, err := yaml.Marshal(&toWrite)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", p, err)
	}
	return nil
}

// Clear 删除配置中的令牌字段并写回
func Clear() error {
	c, err := Load()
	if err != nil {
		return err
	}
	c.AccessToken = ""
	c.RefreshToken = ""
	c.Username = ""
	return Save(c)
}
