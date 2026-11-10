package route

import (
	"encoding/json"
	"net/http/httptest"
	"shadmin/domain"
	"shadmin/frontend"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFallbackContract(t *testing.T) {
	for _, production := range []bool{false, true} {
		t.Run(map[bool]string{false: "dev", true: "prod"}[production], func(t *testing.T) {
			r := gin.New()
			if err := DefaultServerConfig().Apply(r); err != nil {
				t.Fatal(err)
			}
			if production {
				frontend.Register(r)
			}
			r.GET("/api/v1/known", func(c *gin.Context) { c.JSON(200, domain.RespSuccess(nil)) })
			for _, req := range []struct {
				method, path string
				status       int
			}{{"GET", "/api/v1/missing", 404}, {"POST", "/api/v1/known", 405}} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(req.method, req.path, nil))
				var body domain.Response
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if w.Code != req.status || body.Code != 1 || body.Msg == "" {
					t.Fatalf("%s %s: %d %s", req.method, req.path, w.Code, w.Body)
				}
			}
		})
	}
}
