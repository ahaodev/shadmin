package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"shadmin/domain"
	"shadmin/internal/cacher"
)

const identityCodeNamespace = "auth:identity:code"

// UserIdentityCodeStore 保存 OAuth 回调后的短期一次性 code。
// 底层复用统一 cacher，自动支持 memory/redis 两种后端。
type UserIdentityCodeStore struct {
	cacher cacher.Cacher
	ttl    time.Duration
}

func NewUserIdentityCodeStore(cacher cacher.Cacher, ttl time.Duration) *UserIdentityCodeStore {
	if cacher == nil {
		panic("user identity code store: cacher is required")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &UserIdentityCodeStore{cacher: cacher, ttl: ttl}
}

func (s *UserIdentityCodeStore) Put(ctx context.Context, result *domain.UserIdentityResult) (string, error) {
	if result == nil {
		return "", errors.New("identity login result is nil")
	}

	code, err := randomCode(24)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	if err := s.cacher.Set(ctx, identityCodeNamespace, code, string(payload), s.ttl); err != nil {
		return "", err
	}
	return code, nil
}

func (s *UserIdentityCodeStore) Consume(ctx context.Context, code string) (*domain.UserIdentityResult, error) {
	if code == "" {
		return nil, nil
	}

	payload, ok, err := s.cacher.GetAndDelete(ctx, identityCodeNamespace, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	var result domain.UserIdentityResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func randomCode(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
