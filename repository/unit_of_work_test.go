package repository

import (
	"context"
	"errors"
	"testing"

	"shadmin/domain"
)

// 验证 UnitOfWork 经 ctx 传递事务：事务内的仓储写入随 Do 一起提交或回滚。
func TestUnitOfWorkRollsBackRepositoryWritesOnError(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	roles := NewRoleRepository(client)
	uow := NewUnitOfWork(client)
	wantErr := errors.New("rollback")

	err := uow.Do(ctx, func(txCtx context.Context) error {
		if err := roles.Create(txCtx, &domain.Role{Name: "operator", Status: domain.RoleStatusActive}); err != nil {
			return err
		}
		// 事务内可见：读取走同一个事务。
		if exists, err := roles.ExistsByName(txCtx, "operator"); err != nil || !exists {
			t.Fatalf("role inside transaction: exists = %v, err = %v; want true", exists, err)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Do error = %v, want %v", err, wantErr)
	}

	if exists, err := roles.ExistsByName(ctx, "operator"); err != nil || exists {
		t.Fatalf("role after rollback: exists = %v, err = %v; want absent", exists, err)
	}
	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 0 {
		t.Fatalf("generation after rollback = %d, err = %v; want 0", got, err)
	}
}

// 事务内的多次授权变更与 WithAuthorizationTx 共享同一次代数推进。
func TestUnitOfWorkCommitsAndSharesOneGenerationBump(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	ctx := context.Background()
	roles := NewRoleRepository(client)
	uow := NewUnitOfWork(client)

	err := uow.Do(ctx, func(txCtx context.Context) error {
		if err := roles.Create(txCtx, &domain.Role{Name: "operator", Status: domain.RoleStatusActive}); err != nil {
			return err
		}
		return roles.Create(txCtx, &domain.Role{Name: "auditor", Status: domain.RoleStatusActive})
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	for _, name := range []string{"operator", "auditor"} {
		if exists, err := roles.ExistsByName(ctx, name); err != nil || !exists {
			t.Fatalf("role %q after commit: exists = %v, err = %v; want true", name, exists, err)
		}
	}
	if got, err := CurrentAuthorizationGeneration(ctx, client); err != nil || got != 1 {
		t.Fatalf("generation = %d, err = %v; want exactly one bump", got, err)
	}
}
