package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"shadmin/domain"

	"github.com/casbin/casbin/v3"
)

const snapshotBuildAttempts = 3

var errAuthorizationChanged = errors.New("authorization data changed while building casbin snapshot")

// SyncService projects the authoritative authorization facts into an in-memory Casbin snapshot.
type SyncService struct {
	source  domain.AuthorizationSource
	manager Manager
	mu      sync.Mutex
}

func NewSyncService(source domain.AuthorizationSource, manager Manager) *SyncService {
	return &SyncService{source: source, manager: manager}
}

// SyncFromDatabase rebuilds the complete snapshot from the authoritative data source.
func (s *SyncService) SyncFromDatabase(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuildLocked(ctx)
}

// SyncIfChanged checks the shared generation and rebuilds only when this process is stale.
func (s *SyncService) SyncIfChanged(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	generation, err := s.currentGeneration(ctx)
	if err != nil {
		s.manager.MarkSnapshotStale()
		return err
	}
	if applied, ok := s.manager.CurrentGeneration(); ok && applied == generation {
		s.manager.MarkSnapshotFresh(generation)
		return nil
	}
	return s.rebuildLocked(ctx)
}

func (s *SyncService) rebuildLocked(ctx context.Context) error {
	s.manager.MarkSnapshotStale()
	started := time.Now()
	for range snapshotBuildAttempts {
		generation, err := s.currentGeneration(ctx)
		if err != nil {
			return err
		}

		enforcer, counts, err := s.buildEnforcer(ctx)
		if err != nil {
			return err
		}

		currentGeneration, err := s.currentGeneration(ctx)
		if err != nil {
			return err
		}
		if currentGeneration != generation {
			continue
		}

		if err := s.manager.ReplaceSnapshot(enforcer, generation); err != nil {
			return fmt.Errorf("publish casbin snapshot: %w", err)
		}
		publishedGeneration, err := s.currentGeneration(ctx)
		if err != nil {
			return err
		}
		if publishedGeneration != generation {
			continue
		}
		if !s.manager.MarkSnapshotFresh(generation) {
			return fmt.Errorf("casbin snapshot generation changed before publication completed")
		}
		logger.Infof("Casbin snapshot published: generation=%d, users=%d, roles=%d, policies=%d, elapsed=%v",
			generation, counts.users, counts.roles, counts.policies, time.Since(started))
		return nil
	}
	return errAuthorizationChanged
}

func (s *SyncService) currentGeneration(ctx context.Context) (int64, error) {
	return s.source.Generation(ctx)
}

type snapshotCounts struct {
	users    int
	roles    int
	policies int
}

func (s *SyncService) buildEnforcer(ctx context.Context) (*casbin.SyncedEnforcer, snapshotCounts, error) {
	enforcer, err := newEnforcer()
	if err != nil {
		return nil, snapshotCounts{}, fmt.Errorf("create snapshot enforcer: %w", err)
	}

	bindings, err := s.source.ActiveUserRoles(ctx)
	if err != nil {
		return nil, snapshotCounts{}, err
	}
	for _, b := range bindings {
		for _, roleID := range b.RoleIDs {
			if _, err := enforcer.AddRoleForUser(b.UserID, roleID); err != nil {
				return nil, snapshotCounts{}, fmt.Errorf("add role %s for user %s: %w", roleID, b.UserID, err)
			}
		}
	}

	roles, err := s.source.ActiveRoles(ctx)
	if err != nil {
		return nil, snapshotCounts{}, err
	}

	policyCount := 0
	for _, r := range roles {
		for _, rule := range desiredRolePolicies(r.Name, r.Resources) {
			if _, err := enforcer.AddNamedPolicy("p", r.ID, rule.obj, rule.act); err != nil {
				return nil, snapshotCounts{}, fmt.Errorf("add policy for role %s: %w", r.ID, err)
			}
			policyCount++
		}
	}

	return enforcer, snapshotCounts{users: len(bindings), roles: len(roles), policies: policyCount}, nil
}

// policyRule is the object/action portion of a Casbin p rule.
type policyRule struct {
	obj string
	act string
}

// desiredRolePolicies computes the p rules for one role. Keep existing semantics:
// the admin role receives the wildcard policy, and public API resources are omitted.
func desiredRolePolicies(roleName string, resources []domain.AuthorizedResource) []policyRule {
	if roleName == "admin" {
		return []policyRule{{obj: "*", act: "*"}}
	}

	rules := make([]policyRule, 0)
	seen := make(map[policyRule]struct{})
	for _, res := range resources {
		if res.IsPublic {
			continue
		}
		rule := policyRule{obj: res.Path, act: res.Method}
		if _, ok := seen[rule]; ok {
			continue
		}
		seen[rule] = struct{}{}
		rules = append(rules, rule)
	}
	return rules
}
