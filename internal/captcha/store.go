package captcha

import "time"

// challengeRecord 单次滑块挑战的服务端状态。JSON 序列化用于 Redis 持久化。
type challengeRecord struct {
	X         int       `json:"x"`
	Y         int       `json:"y"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	Attempts  int       `json:"attempts"`
}

// ChallengeStore 抽象 challenge 存储，支持进程内存与 Redis 两种实现。
// 内存模式由后台 goroutine 清理过期项；Redis 模式依赖 key TTL。
type ChallengeStore interface {
	// Save 写入（或覆盖）一条 challenge，ttl 为相对有效期。
	Save(id string, rec challengeRecord, ttl time.Duration) error
	// Load 读取一条 challenge；ok=false 表示不存在。
	Load(id string) (challengeRecord, bool, error)
	// Delete 删除一条 challenge（允许重复删除）。
	Delete(id string) error
	// Close 释放底层资源（如清理 goroutine）。
	Close() error
}
