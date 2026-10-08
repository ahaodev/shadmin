package domain

import "context"

// UnitOfWork 在同一个数据库事务内执行多个仓储操作（由 repository 实现）。
// 事务经 ctx 传递：fn 收到的 ctx 携带事务，参与事务的仓储方法必须使用该 ctx。
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
