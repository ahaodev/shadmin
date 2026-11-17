package domain

import "context"

// UnitOfWork 在同一事务内执行 fn；仓储操作必须使用 fn 收到的 ctx。
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
