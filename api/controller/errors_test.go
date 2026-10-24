package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

func respond(t *testing.T, err error) (int, domain.Response) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/probe", nil)

	RespondError(c, err)

	var body domain.Response
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), e)
	}
	return w.Code, body
}

// 每条登记的哨兵错误都映射到声明的状态码；包装后（%w）依然命中。
func TestRespondError_MapsEveryRegisteredSentinel(t *testing.T) {
	for _, rule := range errorRules {
		for name, err := range map[string]error{
			"bare":    rule.err,
			"wrapped": fmt.Errorf("usecase: %w", rule.err),
		} {
			t.Run(rule.err.Error()+"/"+name, func(t *testing.T) {
				code, body := respond(t, err)
				if code != rule.status {
					t.Fatalf("status = %d, want %d", code, rule.status)
				}
				if body.Code != domain.FailCode {
					t.Fatalf("body code = %d, want %d", body.Code, domain.FailCode)
				}
				if rule.msg != "" && body.Msg != rule.msg {
					t.Fatalf("msg = %q, want fixed %q", body.Msg, rule.msg)
				}
				if rule.msg == "" && !strings.Contains(body.Msg, rule.err.Error()) {
					t.Fatalf("msg = %q, want it to contain %q", body.Msg, rule.err.Error())
				}
			})
		}
	}
}

// 同一个哨兵错误不能登记两次（先登记者会遮蔽后者，行为难以察觉）。
func TestErrorRules_NoDuplicates(t *testing.T) {
	seen := map[error]bool{}
	for _, r := range errorRules {
		if seen[r.err] {
			t.Fatalf("duplicate rule for %v", r.err)
		}
		seen[r.err] = true
	}
}

// 未登记的错误一律 500，且不回传内部细节（SQL / 驱动信息）。
func TestRespondError_UnknownErrorIsGeneric500(t *testing.T) {
	secret := errors.New(`pq: duplicate key value violates unique constraint "users_username_key"`)

	code, body := respond(t, fmt.Errorf("create user: %w", secret))

	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if strings.Contains(body.Msg, "pq:") || strings.Contains(body.Msg, "users_username_key") {
		t.Fatalf("500 body leaks internal error: %q", body.Msg)
	}
}

// 账户锁定：423，保留面向用户的提示。
func TestRespondError_AccountLocked(t *testing.T) {
	code, body := respond(t, fmt.Errorf("login: %w", &domain.AccountLockedError{}))

	if code != http.StatusLocked {
		t.Fatalf("status = %d, want 423", code)
	}
	if body.Msg != (&domain.AccountLockedError{}).Error() {
		t.Fatalf("msg = %q", body.Msg)
	}
}

// 登录失败带"还可尝试 N 次"的上下文，不能被映射表吞掉。
func TestRespondError_KeepsWrappedContextWhenNoFixedMessage(t *testing.T) {
	err := fmt.Errorf("%w，还可尝试 %d 次", domain.ErrInvalidCredentials, 3)

	code, body := respond(t, err)

	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
	if body.Msg != err.Error() {
		t.Fatalf("msg = %q, want %q", body.Msg, err.Error())
	}
}
