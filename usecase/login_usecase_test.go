package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"shadmin/domain"
)

type loginTestRepository struct {
	domain.UserRepository
	getByIdentifierErr error
}

func (r *loginTestRepository) GetByIdentifier(context.Context, string) (*domain.User, error) {
	return nil, r.getByIdentifierErr
}

type loginTestCaptcha struct{ domain.CaptchaUsecase }

func (loginTestCaptcha) VerifySlide(context.Context, string, int, int) error { return nil }

type loginTestSecurity struct {
	failures int
}

func (s *loginTestSecurity) IsLocked(context.Context, string) bool { return false }
func (s *loginTestSecurity) RecordFailedAttempt(context.Context, string) int {
	s.failures++
	return s.failures
}
func (s *loginTestSecurity) RecordSuccessfulLogin(context.Context, string) {}
func (s *loginTestSecurity) MaxFailures() int                              { return 5 }

func newLoginTestUsecase(repo domain.UserRepository, security domain.LoginSecurity) domain.LoginUsecase {
	return NewLoginUsecase(repo, loginTestCaptcha{}, nil, security, nil, nil, time.Second)
}

// 用户不存在：算作凭证错误并累计失败次数（避免账号枚举）。
func TestLogin_UserNotFoundIsInvalidCredentials(t *testing.T) {
	security := &loginTestSecurity{}
	uc := newLoginTestUsecase(&loginTestRepository{getByIdentifierErr: domain.ErrUserNotFound}, security)

	_, err := uc.Login(context.Background(), &domain.LoginRequest{Identifier: "ghost", Password: "x", CaptchaID: "c"}, domain.LoginMeta{})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
	if security.failures != 1 {
		t.Fatalf("failed attempts recorded = %d, want 1", security.failures)
	}
}

// 数据库故障不能被当作"用户名或密码错误"，也不能给真实用户累计失败次数而被锁定。
func TestLogin_RepositoryFailureIsNotCredentialError(t *testing.T) {
	security := &loginTestSecurity{}
	dbErr := errors.New("connection refused")
	uc := newLoginTestUsecase(&loginTestRepository{getByIdentifierErr: dbErr}, security)

	_, err := uc.Login(context.Background(), &domain.LoginRequest{Identifier: "alice", Password: "x", CaptchaID: "c"}, domain.LoginMeta{})

	if errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, must not be reported as invalid credentials", err)
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want it to wrap the repository error", err)
	}
	if security.failures != 0 {
		t.Fatalf("failed attempts recorded = %d, want 0 (outage must not lock accounts)", security.failures)
	}
}
