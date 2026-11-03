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

// fakeManager 只实现 CheckPermission：中间件测试只关心网关行为
// （状态码、userID 提取、所有路径均检查），权限判定本身由 internal/casbin 的单测覆盖。
type fakeManager struct {
	domain.PermissionChecker // Unused methods are never called.
	allow                    bool
	err                      error
}

func (f fakeManager) CheckPermission(context.Context, string, string, string) (bool, error) {
	return f.allow, f.err
}

// newCasbinTestRouter 构造三条约目的路由：
// /api/v1/probe   带用户 alice；
// /api/v1/anon    匿名（userID 为空）；
// /api/v1/profile 也不能绕过显式挂载的权限校验。
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

	// 带用户但无权限
	if w := casbinProbe(t, r, "/api/v1/probe"); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized user status = %d, want 403", w.Code)
	}
	// 匿名（userID 为空）不放过
	if w := casbinProbe(t, r, "/api/v1/anon"); w.Code != http.StatusForbidden {
		t.Fatalf("anonymous status = %d, want 403", w.Code)
	}
}

// Path names must not bypass a mounted authorization gate.
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
