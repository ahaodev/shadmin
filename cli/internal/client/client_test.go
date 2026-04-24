package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"shadmin-cli/internal/clierr"
	"shadmin-cli/internal/config"
)

func respEnvelope(t *testing.T, code int, msg string, data any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"code": code, "msg": msg, "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cli, err := New(&config.Config{ServerURL: srv.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	cli.DisablePersist()
	return srv, cli
}

func TestDoSuccess(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respEnvelope(t, 0, "OK", map[string]string{"id": "u1", "name": "alice"}))
	})
	var out map[string]string
	if err := cli.Do(context.Background(), "GET", "/api/v1/system/user/u1", nil, nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out["name"] != "alice" {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestDoBusinessError(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(respEnvelope(t, 1, "bad input", nil))
	})
	err := cli.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if clierr.ExitCode(err) != clierr.ExitGeneric {
		t.Errorf("expected ExitGeneric, got %d", clierr.ExitCode(err))
	}
}

func TestDoForbidden(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write(respEnvelope(t, 1, "forbidden", nil))
	})
	err := cli.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if clierr.ExitCode(err) != clierr.ExitForbidden {
		t.Errorf("expected ExitForbidden, got %d (err=%v)", clierr.ExitCode(err), err)
	}
}

func TestDoNotFound(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write(respEnvelope(t, 1, "not found", nil))
	})
	err := cli.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if clierr.ExitCode(err) != clierr.ExitNotFound {
		t.Errorf("expected ExitNotFound, got %d", clierr.ExitCode(err))
	}
}

func TestDo401RefreshSucceeds(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			w.WriteHeader(http.StatusOK)
			w.Write(respEnvelope(t, 0, "OK", map[string]string{
				"accessToken": "new-access", "refreshToken": "new-refresh",
			}))
		default:
			// 第一次 401，第二次（带新 token）200
			if r.Header.Get("Authorization") == "Bearer new-access" {
				w.WriteHeader(http.StatusOK)
				w.Write(respEnvelope(t, 0, "OK", map[string]string{"ok": "yes"}))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(respEnvelope(t, 1, "unauthorized", nil))
		}
	}))
	t.Cleanup(srv.Close)

	cli, _ := New(&config.Config{ServerURL: srv.URL, AccessToken: "old", RefreshToken: "r"})
	cli.DisablePersist()

	var out map[string]string
	if err := cli.Do(context.Background(), "GET", "/api/v1/protected", nil, nil, &out); err != nil {
		t.Fatalf("expected success after refresh, got %v", err)
	}
	if out["ok"] != "yes" {
		t.Errorf("unexpected body: %+v", out)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 calls (401 + refresh + retry), got %d", calls)
	}
}

func TestDo401NoRefreshToken(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(respEnvelope(t, 1, "unauthorized", nil))
	})
	// 清掉 refresh token，避免走 refresh 分支
	cli.cfg.RefreshToken = ""
	err := cli.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if clierr.ExitCode(err) != clierr.ExitUnauth {
		t.Errorf("expected ExitUnauth, got %d (err=%v)", clierr.ExitCode(err), err)
	}
}

func TestDoServerError(t *testing.T) {
	_, cli := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`boom`))
	})
	err := cli.Do(context.Background(), "GET", "/x", nil, nil, nil)
	if clierr.ExitCode(err) != clierr.ExitServerError {
		t.Errorf("expected ExitServerError, got %d", clierr.ExitCode(err))
	}
}
