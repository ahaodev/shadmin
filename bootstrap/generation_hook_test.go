package bootstrap

import (
	"context"
	"testing"
	"time"

	"shadmin/internal/casbin"
	"shadmin/internal/constants"
	schedulers "shadmin/internal/scheduler"
	"shadmin/repository"
)

func TestAuthorizationGenerationHookTriggersAfterCommit(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	manager := casbin.NewCasManager()
	syncService := casbin.NewSyncService(client, manager)
	scheduler := schedulers.NewCasbinSyncScheduler(syncService, time.Hour)
	app := &Application{DB: client, CasbinScheduler: scheduler}
	app.registerAuthorizationGenerationHook()
	scheduler.Start(ctx)
	t.Cleanup(scheduler.Stop)

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if err := tx.AuthzState.UpdateOneID(constants.AuthorizationStateID).AddGeneration(1).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("bump generation: %v", err)
	}
	if _, loaded := manager.CurrentGeneration(); loaded {
		t.Fatal("snapshot was refreshed before authorization transaction committed")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if generation, loaded := manager.CurrentGeneration(); loaded && generation == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	generation, loaded := manager.CurrentGeneration()
	t.Fatalf("generation trigger did not refresh snapshot; generation=%d loaded=%v", generation, loaded)
}

func TestAuthorizationGenerationHookDoesNotTriggerOnRollback(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	manager := casbin.NewCasManager()
	syncService := casbin.NewSyncService(client, manager)
	scheduler := schedulers.NewCasbinSyncScheduler(syncService, time.Hour)
	app := &Application{DB: client, CasbinScheduler: scheduler}
	app.registerAuthorizationGenerationHook()
	scheduler.Start(ctx)
	t.Cleanup(scheduler.Stop)

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if err := tx.AuthzState.UpdateOneID(constants.AuthorizationStateID).AddGeneration(1).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("bump generation: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if _, loaded := manager.CurrentGeneration(); loaded {
		t.Fatal("generation trigger ran for a rolled-back transaction")
	}
}
