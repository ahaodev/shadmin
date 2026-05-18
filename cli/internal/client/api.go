package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type LoginTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// RequestDeviceCode starts the OAuth device authorization flow.
func (c *Client) RequestDeviceCode(ctx context.Context, clientID, clientName string) (*DeviceCodeResponse, error) {
	body := map[string]string{"client_id": clientID}
	if clientName != "" {
		body["client_name"] = clientName
	}
	var data DeviceCodeResponse
	if err := c.Do(ctx, "POST", "/api/v1/auth/device/code", nil, body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// PollDeviceToken polls for device authorization completion.
func (c *Client) PollDeviceToken(ctx context.Context, clientID, deviceCode string) (*LoginTokenResponse, error) {
	body := map[string]string{
		"client_id":   clientID,
		"device_code": deviceCode,
	}
	var data LoginTokenResponse
	if err := c.Do(ctx, "POST", "/api/v1/auth/device/token", nil, body, &data); err != nil {
		return nil, err
	}
	return &data, nil
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
