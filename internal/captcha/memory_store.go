package captcha

import (
	"sync"
	"time"
)

// memoryStore 进程内 map 存储，后台 goroutine 清理过期 challenge。
type memoryStore struct {
	mu         sync.Mutex
	challenges map[string]challengeRecord
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewMemoryStore() *memoryStore {
	s := &memoryStore{
		challenges: make(map[string]challengeRecord),
		stopCh:     make(chan struct{}),
	}
	go s.startCleanup()
	return s
}

func (s *memoryStore) Save(id string, rec challengeRecord, _ time.Duration) error {
	s.mu.Lock()
	s.challenges[id] = rec
	s.mu.Unlock()
	return nil
}

func (s *memoryStore) Load(id string) (challengeRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.challenges[id]
	return rec, ok, nil
}

func (s *memoryStore) Delete(id string) error {
	s.mu.Lock()
	delete(s.challenges, id)
	s.mu.Unlock()
	return nil
}

func (s *memoryStore) Close() error {
	s.stopOnce.Do(func() { close(s.stopCh) })
	return nil
}

func (s *memoryStore) startCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanupExpired()
		case <-s.stopCh:
			return
		}
	}
}

func (s *memoryStore) cleanupExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.challenges {
		if rec.Used || now.After(rec.ExpiresAt) {
			delete(s.challenges, id)
		}
	}
}
