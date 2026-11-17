package bootstrap

import (
	"context"

	"shadmin/ent"
)

// txMutator 的 Tx 仅在事务驱动下成功。
type txMutator interface {
	Tx() (*ent.Tx, error)
}

func mutationTx(m ent.Mutation) *ent.Tx {
	probe, ok := m.(txMutator)
	if !ok {
		return nil
	}
	tx, err := probe.Tx()
	if err != nil {
		return nil
	}
	return tx
}

// afterCommit 仅在提交成功后执行 fn；非事务变更立即执行。
func afterCommit(m ent.Mutation, fn func()) {
	if tx := mutationTx(m); tx != nil {
		tx.OnCommit(func(next ent.Committer) ent.Committer {
			return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
				if err := next.Commit(ctx, tx); err != nil {
					return err
				}
				fn()
				return nil
			})
		})
		return
	}
	fn()
}

// idMutation 也匹配其他实体；调用方须先过滤实体类型。
type idMutation interface {
	Op() ent.Op
	ID() (string, bool)
	IDs(context.Context) ([]string, error)
}

func mutationIDs(ctx context.Context, m ent.Mutation) ([]string, error) {
	im, ok := m.(idMutation)
	if !ok {
		return nil, nil
	}
	return idsFromMutation(ctx, im.Op(), im.ID, im.IDs)
}

// idsFromMutation 只查 Update/Delete；Create 的默认 ID 需从返回值补齐。
func idsFromMutation(ctx context.Context, op ent.Op, id func() (string, bool), ids func(context.Context) ([]string, error)) ([]string, error) {
	if singleID, ok := id(); ok && singleID != "" {
		return []string{singleID}, nil
	}
	if op.Is(ent.OpCreate) {
		return nil, nil
	}
	return ids(ctx)
}
