package casbin

import (
	"errors"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

var ErrSnapshotStale = errors.New("casbin authorization snapshot is stale")

// Manager 发布只读快照；SyncService 独立构建替代快照。
type Manager interface {
	CheckPermission(userID, object, action string) (bool, error)
	ReplaceSnapshot(enforcer *casbin.SyncedEnforcer, generation int64) error
	CurrentGeneration() (int64, bool)
	SnapshotFresh() bool
	MarkSnapshotStale()
	MarkSnapshotFresh(generation int64) bool
}

type authorizationSnapshot struct {
	enforcer   *casbin.SyncedEnforcer
	generation int64
}

// CasManager 的锁保护快照发布，Enforcer 自行保护并发求值。
type CasManager struct {
	mu       sync.RWMutex
	snapshot *authorizationSnapshot
	stale    bool
}

func NewCasManager() Manager {
	return &CasManager{stale: true}
}

// newEnforcer 禁用 AutoSave，策略完全从应用数据库重建。
func newEnforcer() (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(ModelConf)
	if err != nil {
		return nil, err
	}

	e, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, err
	}
	e.EnableAutoSave(false)
	e.SetLogger(newCasbinLogger())
	return e, nil
}

func (m *CasManager) currentSnapshot() *authorizationSnapshot {
	m.mu.RLock()
	snapshot := m.snapshot
	m.mu.RUnlock()
	return snapshot
}

func (m *CasManager) ReplaceSnapshot(enforcer *casbin.SyncedEnforcer, generation int64) error {
	if enforcer == nil {
		return fmt.Errorf("cannot publish a nil casbin enforcer")
	}
	m.mu.Lock()
	m.snapshot = &authorizationSnapshot{enforcer: enforcer, generation: generation}
	m.mu.Unlock()
	return nil
}

func (m *CasManager) MarkSnapshotStale() {
	m.mu.Lock()
	m.stale = true
	m.mu.Unlock()
}

func (m *CasManager) MarkSnapshotFresh(generation int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snapshot == nil || m.snapshot.generation != generation {
		return false
	}
	m.stale = false
	return true
}

func (m *CasManager) SnapshotFresh() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.stale && m.snapshot != nil
}

func (m *CasManager) CurrentGeneration() (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := m.snapshot
	if snapshot == nil {
		return 0, false
	}
	return snapshot.generation, true
}

func (m *CasManager) CheckPermission(userID, object, action string) (bool, error) {
	m.mu.RLock()
	snapshot := m.snapshot
	stale := m.stale
	m.mu.RUnlock()
	if stale {
		return false, ErrSnapshotStale
	}
	if snapshot == nil || snapshot.enforcer == nil {
		return false, fmt.Errorf("casbin snapshot is not initialized")
	}
	return snapshot.enforcer.Enforce(userID, object, action)
}
