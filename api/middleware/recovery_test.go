package middleware

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"shadmin/domain"
	"strings"
	"testing"
)

func TestRecoveryUsesResponseContract(t *testing.T) {
	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) { panic("private database detail") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))
	var body domain.Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || body.Code != 1 || strings.Contains(w.Body.String(), "private database detail") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
