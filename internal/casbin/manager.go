package casbin

import (
	"errors"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

// Manager exposes authorization checks and snapshot publication. Published
// Enforcers are read-only; SyncService builds replacements off to the side.
var ErrSnapshotStale = errors.New("casbin authorization snapshot is stale")

type Manager interface {
	CheckPermission(userID, object, action string) (bool, error)
	ReplaceSnapshot(enforcer *casbin.SyncedEnforcer, generation int64) error
	CurrentGeneration() (int64, bool)
	MarkSnapshotStale()
	MarkSnapshotFresh(generation int64) bool
}

type authorizationSnapshot struct {
	enforcer   *casbin.SyncedEnforcer
	generation int64
}

// CasManager evaluates requests against an immutable published snapshot. A short
// RWMutex protects pointer publication; the Enforcer itself protects concurrent
// evaluation calls.
type CasManager struct {
	mu       sync.RWMutex
	snapshot *authorizationSnapshot
	stale    bool
}

// NewCasManager creates an in-memory Casbin manager. Shadmin's relational
// authorization data is authoritative; the runtime snapshot is rebuilt from it.
func NewCasManager() Manager {
	return &CasManager{stale: true}
}

// newEnforcer builds an independent Enforcer. Runtime policy snapshots deliberately
// disable AutoSave: the complete policy set is derived from the application database.
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

// ReplaceSnapshot atomically publishes a fully built Enforcer and its source generation.
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

func (m *CasManager) CurrentGeneration() (int64, bool) {
	snapshot := m.currentSnapshot()
	if snapshot == nil {
		return 0, false
	}
	return snapshot.generation, true
}

// CheckPermission evaluates the user against the role-aware Casbin matcher.
// Snapshot construction only creates policies for roles, so Casbin resolves the
// user-to-role relationship directly through the g matcher.
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
