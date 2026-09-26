package casbin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"shadmin/ent"
	"shadmin/ent/role"
	"shadmin/ent/user"

	_ "github.com/mattn/go-sqlite3"
)

func newSnapshotTestService(t *testing.T) (*ent.Client, *CasManager, *SyncService) {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client, err := ent.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := client.AuthzState.Create().SetID("global").Exec(ctx); err != nil {
		t.Fatalf("create authz state: %v", err)
	}

	manager := &CasManager{stale: true}
	return client, manager, NewSyncService(client, manager)
}

func addSnapshotTestData(t *testing.T, client *ent.Client) string {
	t.Helper()
	ctx := context.Background()

	protected, err := client.ApiResource.Create().
		SetID("GET:/api/v1/report").
		SetMethod("GET").
		SetPath("/api/v1/report").
		SetHandler("reportHandler").
		Save(ctx)
	if err != nil {
		t.Fatalf("create protected API resource: %v", err)
	}
	public, err := client.ApiResource.Create().
		SetID("GET:/api/v1/health").
		SetMethod("GET").
		SetPath("/api/v1/health").
		SetHandler("healthHandler").
		SetIsPublic(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create public API resource: %v", err)
	}

	menu, err := client.Menu.Create().
		SetName("Reports").
		SetStatus("active").
		AddAPIResources(protected, public).
		Save(ctx)
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	role, err := client.Role.Create().
		SetName("operator").
		SetStatus("active").
		AddMenus(menu).
		Save(ctx)
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	userEntity, err := client.User.Create().
		SetUsername("alice").
		SetStatus(user.StatusActive).
		AddRoles(role).
		Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return userEntity.ID
}

func TestSyncFromDatabaseBuildsSnapshotFromEntRelations(t *testing.T) {
	client, manager, service := newSnapshotTestService(t)
	userID := addSnapshotTestData(t, client)

	if err := service.SyncFromDatabase(context.Background()); err != nil {
		t.Fatalf("SyncFromDatabase: %v", err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); err != nil || !allowed {
		t.Fatalf("protected permission = %v, err = %v; want allow", allowed, err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "POST"); err != nil || allowed {
		t.Fatalf("wrong method permission = %v, err = %v; want deny", allowed, err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/health", "GET"); err != nil || allowed {
		t.Fatalf("public resource policy = %v, err = %v; want no Casbin grant", allowed, err)
	}

	generation, ok := manager.CurrentGeneration()
	if !ok || generation != 0 {
		t.Fatalf("snapshot generation = %d, loaded = %v; want generation 0", generation, ok)
	}
}

func TestSyncIfChangedPublishesEmptySnapshotAfterUserDeactivation(t *testing.T) {
	client, manager, service := newSnapshotTestService(t)
	userID := addSnapshotTestData(t, client)
	ctx := context.Background()

	if err := service.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("initial SyncFromDatabase: %v", err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); err != nil || !allowed {
		t.Fatalf("initial permission = %v, err = %v; want allow", allowed, err)
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if err := tx.User.UpdateOneID(userID).SetStatus(user.StatusInactive).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("deactivate user: %v", err)
	}
	if err := tx.Role.Update().SetStatus("inactive").Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("deactivate role: %v", err)
	}
	if err := tx.AuthzState.UpdateOneID("global").AddGeneration(1).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("bump generation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	if err := service.SyncIfChanged(ctx); err != nil {
		t.Fatalf("SyncIfChanged: %v", err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); err != nil || allowed {
		t.Fatalf("permission after deactivation = %v, err = %v; want deny", allowed, err)
	}
	if generation, ok := manager.CurrentGeneration(); !ok || generation != 1 {
		t.Fatalf("snapshot generation = %d, loaded = %v; want generation 1", generation, ok)
	}
}

func TestSyncFromDatabasePublishesEmptySnapshot(t *testing.T) {
	client, manager, service := newSnapshotTestService(t)
	userID := addSnapshotTestData(t, client)
	ctx := context.Background()

	if err := service.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("initial SyncFromDatabase: %v", err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); err != nil || !allowed {
		t.Fatalf("initial permission = %v, err = %v; want allow", allowed, err)
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatalf("begin delete transaction: %v", err)
	}
	if err := tx.User.DeleteOneID(userID).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("delete user: %v", err)
	}
	if err := tx.Role.Update().SetStatus("inactive").Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("deactivate role: %v", err)
	}
	if err := tx.AuthzState.UpdateOneID("global").AddGeneration(1).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("bump generation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit generation: %v", err)
	}

	if err := service.SyncIfChanged(ctx); err != nil {
		t.Fatalf("SyncIfChanged: %v", err)
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); err != nil || allowed {
		t.Fatalf("permission after deleting the last user = %v, err = %v; want deny", allowed, err)
	}
	if generation, ok := manager.CurrentGeneration(); !ok || generation != 1 {
		t.Fatalf("snapshot generation = %d, loaded = %v; want generation 1", generation, ok)
	}
}

func TestInstancesIndependentlyApplySharedGeneration(t *testing.T) {
	client, firstManager, firstService := newSnapshotTestService(t)
	_ = addSnapshotTestData(t, client)
	ctx := context.Background()
	if err := firstService.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("initial first sync: %v", err)
	}

	secondManager := &CasManager{stale: true}
	secondService := NewSyncService(client, secondManager)
	if err := secondService.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("initial second sync: %v", err)
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	role, err := tx.Role.Query().Where(role.NameEQ("operator")).Only(ctx)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("load role: %v", err)
	}
	bob, err := tx.User.Create().SetUsername("bob").SetStatus(user.StatusActive).AddRoles(role).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("create second user: %v", err)
	}
	if err := tx.AuthzState.UpdateOneID("global").AddGeneration(1).Exec(ctx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("bump generation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	if err := firstService.SyncIfChanged(ctx); err != nil {
		t.Fatalf("sync first instance: %v", err)
	}
	if err := secondService.SyncIfChanged(ctx); err != nil {
		t.Fatalf("sync second instance: %v", err)
	}
	for name, manager := range map[string]*CasManager{"first": firstManager, "second": secondManager} {
		if allowed, err := manager.CheckPermission(bob.ID, "/api/v1/report", "GET"); err != nil || !allowed {
			t.Fatalf("%s instance permission = %v, err = %v; want allow", name, allowed, err)
		}
		if generation, ok := manager.CurrentGeneration(); !ok || generation != 1 {
			t.Fatalf("%s generation = %d, loaded = %v; want 1", name, generation, ok)
		}
	}
}

func TestSyncIfChangedSkipsCurrentGeneration(t *testing.T) {
	client, manager, service := newSnapshotTestService(t)
	_ = addSnapshotTestData(t, client)
	ctx := context.Background()

	if err := service.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("SyncFromDatabase: %v", err)
	}
	before, loaded := manager.CurrentGeneration()
	if !loaded {
		t.Fatal("snapshot not marked as loaded")
	}
	if err := service.SyncIfChanged(ctx); err != nil {
		t.Fatalf("SyncIfChanged: %v", err)
	}
	after, loaded := manager.CurrentGeneration()
	if !loaded || after != before {
		t.Fatalf("generation after no-op sync = %d, loaded = %v; want %d", after, loaded, before)
	}
}

func TestSyncIfChangedFailsClosedWhenGenerationCannotBeRead(t *testing.T) {
	client, manager, service := newSnapshotTestService(t)
	userID := addSnapshotTestData(t, client)
	ctx := context.Background()
	if err := service.SyncFromDatabase(ctx); err != nil {
		t.Fatalf("initial SyncFromDatabase: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close database client: %v", err)
	}

	if err := service.SyncIfChanged(ctx); err == nil {
		t.Fatal("SyncIfChanged succeeded after the database client was closed")
	}
	if allowed, err := manager.CheckPermission(userID, "/api/v1/report", "GET"); allowed || !errors.Is(err, ErrSnapshotStale) {
		t.Fatalf("CheckPermission after generation read failure = (%v, %v), want (false, ErrSnapshotStale)", allowed, err)
	}
}
