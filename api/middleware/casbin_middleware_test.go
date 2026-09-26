package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"shadmin/internal/casbin"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

// fakeManager 只实现 CheckPermission：中间件测试只关心网关行为
// （状态码、userID 提取、跳过清单），权限判定本身由 internal/casbin 的单测覆盖。
type fakeManager struct {
	casbin.Manager // 嵌入真实接口，未触发的方法不调用
	allow          bool
	err            error
}

func (f fakeManager) CheckPermission(string, string, string) (bool, error) {
	return f.allow, f.err
}

// newCasbinTestRouter 构造三条约目的路由：
// /api/v1/probe   带用户 alice；
// /api/v1/anon    匿名（userID 为空）；
// /api/v1/profile 属于 BasicProtectedAPIPaths，应跳过权限校验。
func newCasbinTestRouter(m casbin.Manager) *gin.Engine {
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

// 跳过路径来自 constants.GetAPIPathsToSkipPermissionCheck，用例锁定这份清单仍生效。
func TestCheckAPIPermission_SkipsBasicProtectedPaths(t *testing.T) {
	r := newCasbinTestRouter(fakeManager{allow: false})

	if w := casbinProbe(t, r, "/api/v1/profile"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (skip paths must bypass permission check)", w.Code)
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
