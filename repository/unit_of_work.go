package repository

import (
	"context"

	"shadmin/domain"
	"shadmin/ent"
)

type entUnitOfWork struct {
	client *ent.Client
}

func NewUnitOfWork(client *ent.Client) domain.UnitOfWork {
	return &entUnitOfWork{client: client}
}

// Do 复用 ctx 中的事务，否则新建事务。
func (u *entUnitOfWork) Do(ctx context.Context, fn func(txCtx context.Context) error) error {
	return withEntTransaction(ctx, u.client, func(txCtx context.Context, _ *ent.Tx) error {
		return fn(txCtx)
	})
}

// clientFromContext 优先使用 ctx 中的事务。
func clientFromContext(ctx context.Context, fallback *ent.Client) *ent.Client {
	if tx := ent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}
