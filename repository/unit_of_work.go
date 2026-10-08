package repository

import (
	"context"

	"shadmin/domain"
	"shadmin/ent"
)

type entUnitOfWork struct {
	client *ent.Client
}

// NewUnitOfWork 构造基于 ent 的事务执行器。事务经 ctx 传递，
// 参与事务的仓储方法通过 clientFromContext 取得事务内的 client。
func NewUnitOfWork(client *ent.Client) domain.UnitOfWork {
	return &entUnitOfWork{client: client}
}

// Do 在事务中执行 fn；若 ctx 已携带事务则加入该事务。
func (u *entUnitOfWork) Do(ctx context.Context, fn func(txCtx context.Context) error) error {
	return withEntTransaction(ctx, u.client, func(txCtx context.Context, _ *ent.Tx) error {
		return fn(txCtx)
	})
}

// clientFromContext 返回 ctx 所携带事务绑定的 client；无事务时返回 fallback。
func clientFromContext(ctx context.Context, fallback *ent.Client) *ent.Client {
	if tx := ent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}
