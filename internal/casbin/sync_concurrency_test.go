package casbin

import (
	"context"
	"shadmin/domain"
	"sync/atomic"
	"testing"
	"time"
)

type delayedSource struct {
	domain.AuthorizationSource
	entered chan struct{}
	release chan struct{}
	reads   atomic.Int32
}

func (s *delayedSource) Generation(ctx context.Context) (int64, error) {
	s.reads.Add(1)
	select {
	case s.entered <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case <-s.release:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
func TestGenerationReadsOverlap(t *testing.T) {
	e, err := newEnforcer()
	if err != nil {
		t.Fatal(err)
	}
	m := NewCasManager()
	if err := m.ReplaceSnapshot(e, 0); err != nil {
		t.Fatal(err)
	}
	m.MarkSnapshotFresh(0)
	source := &delayedSource{entered: make(chan struct{}, 2), release: make(chan struct{})}
	service := NewSyncService(source, m)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- service.SyncIfChanged(ctx) }()
	}
	// Both database reads must enter before either one is released.
	for range 2 {
		select {
		case <-source.entered:
		case <-ctx.Done():
			t.Fatal("generation reads serialized")
		}
	}
	close(source.release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if source.reads.Load() != 2 {
		t.Fatalf("reads = %d", source.reads.Load())
	}
}
func TestRebuildWaitIsCancelable(t *testing.T) {
	s := NewSyncService(nil, NewCasManager())
	if err := s.lock(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer s.unlock()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.SyncFromDatabase(ctx); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}
