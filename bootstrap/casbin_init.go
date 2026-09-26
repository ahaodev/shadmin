package bootstrap

import (
	"context"
	"fmt"
	"time"

	"shadmin/ent"
	"shadmin/internal/casbin"
)

// CasbinInitializer builds the startup policy snapshot and owns the sync service.
type CasbinInitializer struct {
	syncService *casbin.SyncService
	manager     casbin.Manager
}

func NewCasbinInitializer(entClient *ent.Client, manager casbin.Manager) *CasbinInitializer {
	return &CasbinInitializer{
		syncService: casbin.NewSyncService(entClient, manager),
		manager:     manager,
	}
}

func (ci *CasbinInitializer) InitializeCasbin(ctx context.Context) error {
	if ci.manager == nil {
		return fmt.Errorf("casbin manager is not initialized")
	}
	return ci.syncService.SyncFromDatabase(ctx)
}

func (ci *CasbinInitializer) GetSyncService() *casbin.SyncService {
	return ci.syncService
}

// InitCasbin initializes the first snapshot before serving requests, then starts
// the per-process generation trigger with a low-frequency generation poll fallback.
func InitCasbin(app *Application) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := app.CasbinInitializer.InitializeCasbin(ctx); err != nil {
		return fmt.Errorf("initial casbin snapshot failed: %w", err)
	}
	if app.CasbinScheduler != nil {
		app.CasbinScheduler.Start(context.Background())
	}
	return nil
}
