package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/cacher"
	"shadmin/internal/constants"
	"shadmin/internal/tokenutil"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newBlacklist(t *testing.T) auth.JWTBlacklist {
	t.Helper()
	return auth.NewTokenBlacklist(cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute}))
}

func newProbeRouter(secret string, blacklist auth.JWTBlacklist) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/probe", JwtAuthMiddleware(secret, blacklist), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString(constants.UserID)})
	})
	return r
}

func probe(t *testing.T, r *gin.Engine, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newToken(t *testing.T) string {
	t.Helper()
	user := &domain.User{ID: "u-1", Username: "alice", Email: "alice@example.com", Roles: []string{"admin"}}
	token, err := tokenutil.CreateAccessToken(user, testSecret, 60)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	return token
}

// 合法令牌必须带 jti 透传到下游
func TestJwtAuthMiddleware_AcceptsValidToken(t *testing.T) {
	w := probe(t, newProbeRouter(testSecret, newBlacklist(t)), "Bearer "+newToken(t))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var got struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	if got.UserID != "u-1" {
		t.Errorf("user_id = %q, want %q", got.UserID, "u-1")
	}
}

// 未配置黑名单时不校验 jti
func TestJwtAuthMiddleware_AcceptsValidTokenWithoutBlacklist(t *testing.T) {
	w := probe(t, newProbeRouter(testSecret, nil), "Bearer "+newToken(t))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

// 无 jti 的令牌无法吊销，配置了黑名单时必须拒绝
func TestJwtAuthMiddleware_RejectsTokenWithoutJTI(t *testing.T) {
	claims := &domain.JwtCustomClaims{
		ID:        "u-1",
		Name:      "alice",
		Email:     "alice@example.com",
		Issuer:    "shadmin",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	w := probe(t, newProbeRouter(testSecret, newBlacklist(t)), "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestJwtAuthMiddleware_RejectsRevokedToken(t *testing.T) {
	blacklist := newBlacklist(t)
	token := newToken(t)

	claims, err := tokenutil.ParseAccessClaims(token, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessClaims: %v", err)
	}
	if err := blacklist.Add(context.Background(), claims.JTI(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("blacklist.Add: %v", err)
	}

	w := probe(t, newProbeRouter(testSecret, blacklist), "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// 黑名单必须以 jti 为键。曾因字段遮蔽把用户 ID 误当成 jti，
// 导致写入键与查询键不一致、登出完全失效。
func TestJwtAuthMiddleware_BlacklistIsKeyedByJTI(t *testing.T) {
	blacklist := newBlacklist(t)
	token := newToken(t)

	claims, err := tokenutil.ParseAccessClaims(token, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessClaims: %v", err)
	}
	if claims.JTI() == claims.ID {
		t.Fatalf("jti and user id are both %q; the fixture cannot detect a mix-up", claims.JTI())
	}

	r := newProbeRouter(testSecret, blacklist)

	if err := blacklist.Add(context.Background(), claims.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("blacklist.Add(user id): %v", err)
	}
	if w := probe(t, r, "Bearer "+token); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: blacklisting the user id must not revoke the token", w.Code)
	}

	if err := blacklist.Add(context.Background(), claims.JTI(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("blacklist.Add(jti): %v", err)
	}
	if w := probe(t, r, "Bearer "+token); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: blacklisting the jti must revoke the token", w.Code)
	}
}

func TestJwtAuthMiddleware_RejectsBadAuthorizationHeader(t *testing.T) {
	token := newToken(t)

	cases := []struct {
		name   string
		header string
	}{
		{"missing header", ""},
		{"no scheme", token},
		{"wrong scheme", "Basic " + token},
		{"empty token", "Bearer "},
		{"extra segment", "Bearer " + token + " x"},
	}

	r := newProbeRouter(testSecret, newBlacklist(t))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := probe(t, r, tc.header); w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestJwtAuthMiddleware_RejectsTokenSignedWithAnotherSecret(t *testing.T) {
	user := &domain.User{ID: "u-1", Username: "alice"}
	token, err := tokenutil.CreateAccessToken(user, "another-secret-another-secret-32", 60)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}

	w := probe(t, newProbeRouter(testSecret, newBlacklist(t)), "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
