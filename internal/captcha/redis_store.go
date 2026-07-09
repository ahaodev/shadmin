package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore 依托 Redis key TTL 自动过期，无需后台清理 goroutine。
type redisStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisStore(client *redis.Client) *redisStore {
	return &redisStore{client: client, keyPrefix: "captcha:slide:"}
}

func (s *redisStore) Save(id string, rec challengeRecord, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("captcha redis store: ttl must be positive")
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal challenge: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.key(id), b, ttl).Err()
}

func (s *redisStore) Load(id string) (challengeRecord, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, err := s.client.Get(ctx, s.key(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return challengeRecord{}, false, nil
	}
	if err != nil {
		return challengeRecord{}, false, fmt.Errorf("get challenge: %w", err)
	}
	var rec challengeRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return challengeRecord{}, false, fmt.Errorf("unmarshal challenge: %w", err)
	}
	return rec, true, nil
}

func (s *redisStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.client.Del(ctx, s.key(id)).Err()
	return nil
}

func (s *redisStore) Close() error { return nil }

func (s *redisStore) key(id string) string {
	return s.keyPrefix + id
}
