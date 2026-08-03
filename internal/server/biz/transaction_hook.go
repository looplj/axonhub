package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// runAfterCommit publishes an external side effect only after the surrounding
// Ent transaction commits successfully. Calls without a transaction have
// already committed when their mutation returns, so they run immediately.
func runAfterCommit(ctx context.Context, fn func(context.Context)) {
	callbackCtx := context.WithoutCancel(ctx)
	if tx := ent.TxFromContext(ctx); tx != nil {
		tx.OnCommit(func(next ent.Committer) ent.Committer {
			return ent.CommitFunc(func(commitCtx context.Context, tx *ent.Tx) error {
				if err := next.Commit(commitCtx, tx); err != nil {
					return err
				}

				fn(callbackCtx)
				return nil
			})
		})
		return
	}

	fn(callbackCtx)
}
