package route

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"shadmin/domain"
	"shadmin/frontend"
	"testing"
)

func TestFallbackContract(t *testing.T) {
	for _, production := range []bool{false, true} {
		t.Run(map[bool]string{false: "dev", true: "prod"}[production], func(t *testing.T) {
			r := gin.New()
			var fallback gin.HandlerFunc
			if production {
				fallback = frontend.Register(r)
			}
			setupFallback(r, fallback)
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
