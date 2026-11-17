package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"shadmin/domain"
	"testing"

	"shadmin/internal/casbin"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

type fakeManager struct {
	domain.PermissionChecker // Unused methods are never called.
	allow                    bool
	err                      error
}

func (f fakeManager) CheckPermission(context.Context, string, string, string) (bool, error) {
	return f.allow, f.err
}

func newCasbinTestRouter(m domain.PermissionChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	withUser := func(userID string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set(constants.UserID, userID)
			c.Next()
		}
	}

	r.GET("/api/v1/probe", withUser("alice"), NewCasbinMiddleware(m).CheckAPIPermission(), ok)
	r.GET("/api/v1/anon", withUser(""), NewCasbinMiddleware(m).CheckAPIPermission(), ok)
	r.GET("/api/v1/profile", NewCasbinMiddleware(m).CheckAPIPermission(), ok)
	return r
}

func casbinProbe(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCheckAPIPermission_AllowsAuthorizedUser(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{allow: true})

	if w := casbinProbe(t, r, "/api/v1/probe"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCheckAPIPermission_DeniesWithoutPermission(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{allow: false})

	if w := casbinProbe(t, r, "/api/v1/probe"); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized user status = %d, want 403", w.Code)
	}
	if w := casbinProbe(t, r, "/api/v1/anon"); w.Code != http.StatusForbidden {
		t.Fatalf("anonymous status = %d, want 403", w.Code)
	}
}

func TestCheckAPIPermission_DoesNotSkipPaths(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{allow: false})

	if w := casbinProbe(t, r, "/api/v1/profile"); w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (no path exemptions)", w.Code)
	}
}

func TestCheckAPIPermission_Returns500OnManagerError(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{err: errors.New("boom")})

	if w := casbinProbe(t, r, "/api/v1/probe"); w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestCheckAPIPermission_FailsClosedWhenSnapshotIsStale(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{err: casbin.ErrSnapshotStale})

	if w := casbinProbe(t, r, "/api/v1/probe"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
