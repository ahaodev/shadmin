package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/config"
)

// Response 对应后端 domain.Response
type Response struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Client 是 Shadmin CLI 的 HTTP 客户端
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	cfg        *config.Config
	persistCfg bool
}

// New 从 config 构建 client
func New(cfg *config.Config) (*Client, error) {
	if cfg == nil || cfg.ServerURL == "" {
		return nil, clierr.New(clierr.ExitUnauth, "server url not configured; run 'shadmin-cli login --server URL' first")
	}
	base := strings.TrimRight(cfg.ServerURL, "/")
	return &Client{
		BaseURL:    base,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
		persistCfg: true,
	}, nil
}

// NewUnauth 用于 login 等未认证场景
func NewUnauth(serverURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(serverURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		cfg:     &config.Config{ServerURL: serverURL},
	}
}

// DisablePersist 禁止 client 自动保存 config（供测试）
func (c *Client) DisablePersist() { c.persistCfg = false }

// Do 执行请求并解包 domain.Response
// out 可为 nil（忽略 data）；out 传 *T 时，data 会被反序列化到 *T
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doWithRetry(ctx, method, path, query, body, out, true)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, query url.Values, body any, out any, allowRefresh bool) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return clierr.Wrap(clierr.ExitGeneric, err, "marshal request")
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return clierr.Wrap(clierr.ExitGeneric, err, "build request")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "shadmin-cli")
	if c.cfg != nil && c.cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return clierr.Wrap(clierr.ExitNetwork, err, "http request failed")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return clierr.Wrap(clierr.ExitNetwork, err, "read response")
	}

	// 401 -> 尝试 refresh 一次
	if resp.StatusCode == http.StatusUnauthorized && allowRefresh && c.cfg != nil && c.cfg.RefreshToken != "" {
		if err := c.refresh(ctx); err == nil {
			return c.doWithRetry(ctx, method, path, query, body, out, false)
		}
		// refresh 失败，清理 token
		c.cfg.AccessToken = ""
		c.cfg.RefreshToken = ""
		if c.persistCfg {
			_ = config.Save(c.cfg)
		}
		return clierr.New(clierr.ExitUnauth, "authentication expired; please run 'shadmin-cli login' again")
	}

	if resp.StatusCode == http.StatusForbidden {
		return clierr.New(clierr.ExitForbidden, "permission denied (403): current user lacks RBAC permission for this API")
	}
	if resp.StatusCode == http.StatusNotFound {
		return clierr.New(clierr.ExitNotFound, fmt.Sprintf("not found (404): %s", path))
	}
	if resp.StatusCode >= 500 {
		return clierr.New(clierr.ExitServerError, fmt.Sprintf("server error %d: %s", resp.StatusCode, truncate(string(raw), 200)))
	}

	var envelope Response
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return clierr.New(clierr.ExitServerError, fmt.Sprintf("decode response: %v (status=%d, body=%s)", err, resp.StatusCode, truncate(string(raw), 200)))
	}
	if envelope.Code != 0 {
		code := clierr.ExitGeneric
		if resp.StatusCode == http.StatusUnauthorized {
			code = clierr.ExitUnauth
		}
		msg := envelope.Msg
		if msg == "" {
			msg = fmt.Sprintf("server returned code=%d", envelope.Code)
		}
		return clierr.New(code, msg)
	}

	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return clierr.Wrap(clierr.ExitGeneric, err, "decode data")
		}
	}
	return nil
}

// refresh 调用 /auth/refresh 刷新 token
func (c *Client) refresh(ctx context.Context) error {
	body := map[string]string{"refreshToken": c.cfg.RefreshToken}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/auth/refresh", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh status %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var env Response
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if env.Code != 0 {
		return fmt.Errorf("refresh failed: %s", env.Msg)
	}
	var tk struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(env.Data, &tk); err != nil {
		return err
	}
	c.cfg.AccessToken = tk.AccessToken
	c.cfg.RefreshToken = tk.RefreshToken
	if c.persistCfg {
		return config.Save(c.cfg)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
