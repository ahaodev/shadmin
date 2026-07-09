package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"shadmin/internal/cachex"
)

// challengeRecord 单次滑块挑战的服务端状态。JSON 序列化用于底层 cachex.Cacher 持久化。
type challengeRecord struct {
	X         int       `json:"x"`
	Y         int       `json:"y"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	Attempts  int       `json:"attempts"`
}

// ChallengeStore 抽象 challenge 存储。
// 内存/Redis 的后端选择由调用方通过注入的 cachex.Cacher 一次性决定：
// 内存 cacher 依赖 go-cache 的 TTL 自动回收，Redis cacher 依赖 key TTL。
type ChallengeStore interface {
	// Save 写入（或覆盖）一条 challenge，ttl 为相对有效期。
	Save(id string, rec challengeRecord, ttl time.Duration) error
	// Load 读取一条 challenge；ok=false 表示不存在。
	Load(id string) (challengeRecord, bool, error)
	// Delete 删除一条 challenge（允许重复删除）。
	Delete(id string) error
	// Close 释放底层资源。
	Close() error
}

const captchaNS = "captcha"

// NewStore 返回基于 cachex.Cacher 的 ChallengeStore 实现。
func NewStore(cacher cachex.Cacher) ChallengeStore {
	return &store{cacher: cacher}
}

type store struct {
	cacher cachex.Cacher
}

func (s *store) Save(id string, rec challengeRecord, ttl time.Duration) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal challenge: %w", err)
	}
	return s.cacher.Set(context.Background(), captchaNS, id, string(b), ttl)
}

func (s *store) Load(id string) (challengeRecord, bool, error) {
	v, ok, err := s.cacher.Get(context.Background(), captchaNS, id)
	if err != nil {
		return challengeRecord{}, false, fmt.Errorf("get challenge: %w", err)
	}
	if !ok {
		return challengeRecord{}, false, nil
	}
	var rec challengeRecord
	if err := json.Unmarshal([]byte(v), &rec); err != nil {
		return challengeRecord{}, false, fmt.Errorf("unmarshal challenge: %w", err)
	}
	return rec, true, nil
}

func (s *store) Delete(id string) error {
	return s.cacher.Delete(context.Background(), captchaNS, id)
}

func (s *store) Close() error {
	return s.cacher.Close(context.Background())
}
