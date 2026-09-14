package bootstrap

import (
	"context"

	"shadmin/ent"
)

// txMutator 是 ent mutation 在事务内才具备的能力：驱动为 *txDriver 时 Tx() 成功。
type txMutator interface {
	Tx() (*ent.Tx, error)
}

// mutationTx 返回变更所在的事务；不在事务中返回 nil。
func mutationTx(m ent.Mutation) *ent.Tx {
	probe, ok := m.(txMutator)
	if !ok {
		return nil
	}
	tx, err := probe.Tx()
	if err != nil {
		return nil // 非事务变更
	}
	return tx
}

// afterCommit 在变更提交成功后执行 fn；非事务变更立即执行，提交失败不执行。
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

// idMutation 是变更前能自报目标 ID 的 mutation 的方法集。
// 它同样匹配 Department/DictItem 等其它生成类型，调用方须先按类型过滤。
type idMutation interface {
	Op() ent.Op
	ID() (string, bool)
	IDs(context.Context) ([]string, error)
}

// mutationIDs 解析本次变更涉及的目标 ID；不具备该方法集时返回空。
func mutationIDs(ctx context.Context, m ent.Mutation) ([]string, error) {
	im, ok := m.(idMutation)
	if !ok {
		return nil, nil
	}
	return idsFromMutation(ctx, im.Op(), im.ID, im.IDs)
}

// idsFromMutation 解析本次变更的 ID。ent 的 IDs() 只对 Update/Delete 有效，
// 而 Create 的 ID 由 DefaultFunc 生成、变更前不可知，改由 collectValueTarget 从返回值补齐。
func idsFromMutation(ctx context.Context, op ent.Op, id func() (string, bool), ids func(context.Context) ([]string, error)) ([]string, error) {
	if singleID, ok := id(); ok && singleID != "" {
		return []string{singleID}, nil
	}
	if op.Is(ent.OpCreate) {
		return nil, nil
	}
	return ids(ctx)
}
