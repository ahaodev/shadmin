package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"shadmin/bootstrap"
	"shadmin/domain"
	"shadmin/internal/casbin"
	"shadmin/internal/conf"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

type routeAuthorizationSource struct{}

func (routeAuthorizationSource) Generation(context.Context) (int64, error) { return 0, nil }
func (routeAuthorizationSource) ActiveUserRoles(context.Context) ([]domain.UserRoleBinding, error) {
	return nil, nil
}
func (routeAuthorizationSource) ActiveRoles(context.Context) ([]domain.AuthorizedRole, error) {
	return nil, nil
}

func TestAllSystemRoutesInheritAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &bootstrap.Application{
		Env: &conf.Env{}, Usecases: &bootstrap.Usecases{},
		CasbinInitializer: bootstrap.NewCasbinInitializer(routeAuthorizationSource{}, casbin.NewCasManager()),
	}
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(func(c *gin.Context) { c.Set(constants.UserID, "unprivileged"); c.Next() })
	routes := NewProtectedRoutes(NewControllerFactory(app))
	routes.setupSystemRoutes(group, app)
	for _, route := range router.Routes() {
		t.Run(route.Method+route.Path, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, route.Path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusForbidden {
				t.Fatalf("unprivileged request = %d; want 403", response.Code)
			}
		})
	}
}
