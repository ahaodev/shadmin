package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"shadmin/ent"
	"shadmin/ent/role"
	"shadmin/ent/user"
	"shadmin/internal/constants"

	"github.com/casbin/casbin/v3"
)

const snapshotBuildAttempts = 3

var errAuthorizationChanged = errors.New("authorization data changed while building casbin snapshot")

// SyncService projects Ent authorization relationships into an in-memory Casbin snapshot.
type SyncService struct {
	entClient *ent.Client
	manager   Manager
	mu        sync.Mutex
}

func NewSyncService(entClient *ent.Client, manager Manager) *SyncService {
	return &SyncService{entClient: entClient, manager: manager}
}

// SyncFromDatabase rebuilds the complete snapshot from the authoritative Ent data.
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
	state, err := s.entClient.AuthzState.Get(ctx, constants.AuthorizationStateID)
	if err != nil {
		return 0, fmt.Errorf("read authorization generation: %w", err)
	}
	return state.Generation, nil
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

	users, err := s.entClient.User.Query().
		Where(user.StatusEQ("active")).
		WithRoles(func(q *ent.RoleQuery) {
			q.Where(role.StatusEQ("active"))
		}).
		All(ctx)
	if err != nil {
		return nil, snapshotCounts{}, fmt.Errorf("query active user-role relationships: %w", err)
	}
	for _, u := range users {
		for _, r := range u.Edges.Roles {
			if _, err := enforcer.AddRoleForUser(u.ID, r.ID); err != nil {
				return nil, snapshotCounts{}, fmt.Errorf("add role %s for user %s: %w", r.ID, u.ID, err)
			}
		}
	}

	roles, err := s.entClient.Role.Query().
		Where(role.StatusEQ("active")).
		WithMenus(func(q *ent.MenuQuery) {
			q.WithAPIResources()
		}).
		All(ctx)
	if err != nil {
		return nil, snapshotCounts{}, fmt.Errorf("query active role permissions: %w", err)
	}

	policyCount := 0
	for _, r := range roles {
		for _, rule := range desiredRolePolicies(r.Name, r.Edges.Menus) {
			if _, err := enforcer.AddNamedPolicy("p", r.ID, rule.obj, rule.act); err != nil {
				return nil, snapshotCounts{}, fmt.Errorf("add policy for role %s: %w", r.ID, err)
			}
			policyCount++
		}
	}

	return enforcer, snapshotCounts{users: len(users), roles: len(roles), policies: policyCount}, nil
}

// policyRule is the object/action portion of a Casbin p rule.
type policyRule struct {
	obj string
	act string
}

// desiredRolePolicies computes the p rules for one role. Keep existing semantics:
// the admin role receives the wildcard policy, and public API resources are omitted.
func desiredRolePolicies(roleName string, menus []*ent.Menu) []policyRule {
	if roleName == "admin" {
		return []policyRule{{obj: "*", act: "*"}}
	}

	rules := make([]policyRule, 0)
	seen := make(map[policyRule]struct{})
	for _, menu := range menus {
		for _, apiRes := range menu.Edges.APIResources {
			if apiRes.IsPublic {
				continue
			}
			rule := policyRule{obj: apiRes.Path, act: apiRes.Method}
			if _, ok := seen[rule]; ok {
				continue
			}
			seen[rule] = struct{}{}
			rules = append(rules, rule)
		}
	}
	return rules
}
