package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitByIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestsHandled := 0
	router := gin.New()
	router.POST("/limited", RateLimitByIP(1, time.Minute), func(c *gin.Context) {
		requestsHandled++
		c.Status(http.StatusNoContent)
	})

	req := func(remoteAddr string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/limited", nil)
		request.RemoteAddr = remoteAddr
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	if got := req("192.0.2.1:1234").Code; got != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", got, http.StatusNoContent)
	}
	if got := req("192.0.2.1:1234").Code; got != http.StatusTooManyRequests {
		t.Fatalf("second request from same IP status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := req("192.0.2.2:1234").Code; got != http.StatusNoContent {
		t.Fatalf("request from another IP status = %d, want %d", got, http.StatusNoContent)
	}
	if requestsHandled != 2 {
		t.Fatalf("handler calls = %d, want 2", requestsHandled)
	}
}
