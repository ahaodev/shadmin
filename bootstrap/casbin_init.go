package bootstrap

import (
	"context"
	"fmt"
	"time"

	"shadmin/domain"
	"shadmin/internal/casbin"
)

type CasbinInitializer struct {
	syncService *casbin.SyncService
	manager     casbin.Manager
}

func NewCasbinInitializer(source domain.AuthorizationSource, manager casbin.Manager) *CasbinInitializer {
	return &CasbinInitializer{
		syncService: casbin.NewSyncService(source, manager),
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

// InitCasbin 在服务启动前构建快照，再启用提交触发与轮询同步。
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
