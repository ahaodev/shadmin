package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Login 调用 /auth/login，返回 access + refresh token
func (c *Client) Login(ctx context.Context, username, password string) (access, refresh string, err error) {
	body := map[string]string{"username": username, "password": password}
	var data struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.Do(ctx, "POST", "/api/v1/auth/login", nil, body, &data); err != nil {
		return "", "", err
	}
	return data.AccessToken, data.RefreshToken, nil
}

// Logout 调用 /auth/logout
func (c *Client) Logout(ctx context.Context) error {
	body := map[string]string{}
	if c.cfg != nil && c.cfg.RefreshToken != "" {
		body["refresh_token"] = c.cfg.RefreshToken
	}
	return c.Do(ctx, "POST", "/api/v1/auth/logout", nil, body, nil)
}

// Profile 调用 /profile，返回原始 data
func (c *Client) Profile(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.Do(ctx, "GET", "/api/v1/profile/", nil, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ListParams 通用列表参数
type ListParams struct {
	Page     int
	PageSize int
	Keyword  string
	Extra    url.Values
}

func (p ListParams) toValues() url.Values {
	v := url.Values{}
	if p.Extra != nil {
		for k, vs := range p.Extra {
			v[k] = vs
		}
	}
	if p.Page > 0 {
		v.Set("page", strconv.Itoa(p.Page))
	}
	if p.PageSize > 0 {
		v.Set("page_size", strconv.Itoa(p.PageSize))
	}
	if p.Keyword != "" {
		v.Set("keyword", p.Keyword)
	}
	return v
}

// GetJSON 通用 GET，返回 raw data
func (c *Client) GetJSON(ctx context.Context, path string, params ListParams) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.Do(ctx, "GET", path, params.toValues(), nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
