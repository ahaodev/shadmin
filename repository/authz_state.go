package repository

import (
	"context"
	"fmt"
	"time"

	"shadmin/ent"
	"shadmin/ent/authzstate"
	"shadmin/internal/constants"
)

func EnsureAuthorizationState(ctx context.Context, client *ent.Client) error {
	exists, err := client.AuthzState.Query().
		Where(authzstate.ID(constants.AuthorizationStateID)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("check authorization state: %w", err)
	}
	if exists {
		return nil
	}

	_, err = client.AuthzState.Create().
		SetID(constants.AuthorizationStateID).
		Save(ctx)
	if err != nil && ent.IsConstraintError(err) {
		// Another application instance may have initialized the singleton concurrently.
		exists, queryErr := client.AuthzState.Query().
			Where(authzstate.ID(constants.AuthorizationStateID)).
			Exist(ctx)
		if queryErr != nil {
			return fmt.Errorf("verify authorization state after create conflict (%v): %w", err, queryErr)
		}
		if exists {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("create authorization state: %w", err)
	}
	return nil
}

type authorizationTxContextKey struct{}

type authorizationTxState struct {
	tx               *ent.Tx
	generationBumped bool
}

// WithAuthorizationTx 将授权变更与代数推进同事务提交；嵌套调用只推进一次。
func WithAuthorizationTx(ctx context.Context, client *ent.Client, mutate func(context.Context, *ent.Tx) error) error {
	return withEntTransaction(ctx, client, func(txCtx context.Context, tx *ent.Tx) error {
		if err := bumpAuthorizationGenerationOnce(txCtx, tx); err != nil {
			return err
		}
		return mutate(txCtx, tx)
	})
}

func withEntTransaction(ctx context.Context, client *ent.Client, mutate func(context.Context, *ent.Tx) error) error {
	if state, ok := ctx.Value(authorizationTxContextKey{}).(*authorizationTxState); ok {
		return mutate(ctx, state.tx)
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	state := &authorizationTxState{tx: tx}
	txCtx := ent.NewTxContext(context.WithValue(ctx, authorizationTxContextKey{}, state), tx)
	if err := mutate(txCtx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// bumpAuthorizationGenerationOnce 在首次授权变更前锁定并推进代数。
func bumpAuthorizationGenerationOnce(ctx context.Context, tx *ent.Tx) error {
	state, ok := ctx.Value(authorizationTxContextKey{}).(*authorizationTxState)
	if ok && state.generationBumped {
		return nil
	}
	if err := BumpAuthorizationGeneration(ctx, tx); err != nil {
		return err
	}
	if ok {
		state.generationBumped = true
	}
	return nil
}

// BumpAuthorizationGeneration 仅用于未被 WithAuthorizationTx 包装的事务，不参与嵌套去重。
func BumpAuthorizationGeneration(ctx context.Context, tx *ent.Tx) error {
	if err := tx.AuthzState.UpdateOneID(constants.AuthorizationStateID).
		AddGeneration(1).
		SetUpdatedAt(time.Now()).
		Exec(ctx); err != nil {
		return fmt.Errorf("advance authorization generation: %w", err)
	}
	return nil
}

func CurrentAuthorizationGeneration(ctx context.Context, client *ent.Client) (int64, error) {
	state, err := client.AuthzState.Query().
		Where(authzstate.ID(constants.AuthorizationStateID)).
		Only(ctx)
	if err != nil {
		return 0, fmt.Errorf("read authorization generation: %w", err)
	}
	return state.Generation, nil
}
