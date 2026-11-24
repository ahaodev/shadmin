package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shadmin/domain"
	"shadmin/internal/auth"
	"shadmin/internal/constants"

	cacher "github.com/ahaodev/go-cache"

	"github.com/gin-gonic/gin"
)

type stubStatusLoader struct {
	status string
	err    error
	calls  int
}

func (s *stubStatusLoader) GetStatusByID(_ context.Context, _ string) (string, error) {
	s.calls++
	return s.status, s.err
}

func newStatusCache(t *testing.T, loader auth.Loader) *auth.Cache {
	t.Helper()
	return auth.NewUserStatusCacher(loader,
		cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute}), time.Minute)
}

// newUserStateRouter 用 x-user-id 模拟上游认证结果。
func newUserStateRouter(statusCache *auth.Cache) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if id := c.GetHeader(constants.UserID); id != "" {
			c.Set(constants.UserID, id)
		}
	})
	r.GET("/probe", UserStateMiddleware(statusCache), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return r
}

func userStateProbe(t *testing.T, r *gin.Engine, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if userID != "" {
		req.Header.Set(constants.UserID, userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUserStateMiddleware_RejectsMissingUserID(t *testing.T) {
	loader := &stubStatusLoader{status: domain.UserStatusActive}
	r := newUserStateRouter(newStatusCache(t, loader))

	if w := userStateProbe(t, r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if loader.calls != 0 {
		t.Errorf("loader calls = %d, want 0", loader.calls)
	}
}

func TestUserStateMiddleware_SkipsCheckWithoutCache(t *testing.T) {
	r := newUserStateRouter(nil)

	if w := userStateProbe(t, r, "u-1"); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", w.Code, w.Body.String())
	}
}

func TestUserStateMiddleware_AllowsActiveUser(t *testing.T) {
	r := newUserStateRouter(newStatusCache(t, &stubStatusLoader{status: domain.UserStatusActive}))

	if w := userStateProbe(t, r, "u-1"); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", w.Code, w.Body.String())
	}
}

func TestUserStateMiddleware_RejectsInactiveStatuses(t *testing.T) {
	cases := []struct {
		name   string
		status string
	}{
		{"inactive", domain.UserStatusInactive},
		{"invited", domain.UserStatusInvited},
		{"suspended", domain.UserStatusSuspended},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newUserStateRouter(newStatusCache(t, &stubStatusLoader{status: tc.status}))

			if w := userStateProbe(t, r, "u-1"); w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body=%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestUserStateMiddleware_MapsUserErrorsToUnauthorized(t *testing.T) {
	for _, loadErr := range []error{domain.ErrUserDisabled, domain.ErrUserNotFound} {
		t.Run(loadErr.Error(), func(t *testing.T) {
			r := newUserStateRouter(newStatusCache(t, &stubStatusLoader{err: loadErr}))

			if w := userStateProbe(t, r, "u-1"); w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body=%s)", w.Code, w.Body.String())
			}
		})
	}
}

// 回源故障返回 503，既拒绝访问，也避免误判为认证失败。
func TestUserStateMiddleware_ReturnsServiceUnavailableOnLoadFailure(t *testing.T) {
	loader := &stubStatusLoader{err: errors.New("db is down")}
	r := newUserStateRouter(newStatusCache(t, loader))

	w := userStateProbe(t, r, "u-1")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
	if loader.calls != 1 {
		t.Errorf("loader calls = %d, want 1", loader.calls)
	}
}

func TestUserStateMiddleware_CachesStatus(t *testing.T) {
	loader := &stubStatusLoader{status: domain.UserStatusActive}
	r := newUserStateRouter(newStatusCache(t, loader))

	for i := range 3 {
		if w := userStateProbe(t, r, "u-1"); w.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i, w.Code)
		}
	}
	if loader.calls != 1 {
		t.Errorf("loader calls = %d, want 1", loader.calls)
	}
}
