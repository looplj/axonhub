package biz

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/ent"
)

type AbstractService struct {
	db *ent.Client
}

var errServiceOwnedTransactionRequired = errors.New("service-owned transaction required")

// entFromContext returns the Ent client from context if it exists (e.g., in a transaction),
// otherwise returns the default database client.
//
// Note: This method is primarily used for transaction support, NOT for privacy checks.
// Ent's privacy policy is evaluated through the context passed to query/mutation methods,
// regardless of which client is used. Privacy rules check context values like user and project ID
// to enforce access control.
func (a *AbstractService) entFromContext(ctx context.Context) *ent.Client {
	db := ent.FromContext(ctx)
	if db != nil {
		return db
	}

	return a.db
}

func (a *AbstractService) RunInTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	_, err = a.runInTransaction(ctx, fn)

	return err
}

// runInTransaction reports whether it created and closed the transaction.
// Callers returning Ent entities can use this to unwrap only entities whose
// transaction driver was closed here, while preserving caller-owned transactions.
// Caller-owned transactions should register post-commit hooks when the underlying
// *ent.Tx is available; tx-client-only callers remain responsible for side effects.
func (a *AbstractService) runInTransaction(ctx context.Context, fn func(context.Context) error) (owned bool, err error) {
	return a.runInTransactionWithOwnership(ctx, func(ctx context.Context, _ bool) error {
		return fn(ctx)
	})
}

// runInTransactionWithOwnership invokes fn with whether this service owns the
// transaction. The ownership signal lets operations whose contract requires an
// independent commit reject caller-owned transactions before making any writes.
func (a *AbstractService) runInTransactionWithOwnership(
	ctx context.Context,
	fn func(context.Context, bool) error,
) (owned bool, err error) {
	if tx := ent.TxFromContext(ctx); tx != nil {
		txClient := tx.Client()
		txCtx := ent.NewContext(ctx, txClient)

		return false, fn(txCtx, false)
	}

	db := a.entFromContext(ctx)

	tx, err := db.Tx(ctx)
	if err != nil {
		// If the client is already transactional (e.g., from tx.Client()),
		// just run the function with the existing transactional client.
		if errors.Is(err, ent.ErrTxStarted) {
			return false, fn(ent.NewContext(ctx, db), false)
		}

		return false, err
	}

	committed := false

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()

			panic(r)
		}

		if !committed {
			_ = tx.Rollback()
		}
	}()

	txClient := tx.Client()
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, txClient)

	if err := fn(txCtx, true); err != nil {
		return true, err
	}

	if err := tx.Commit(); err != nil {
		return true, err
	}

	committed = true

	return true, nil
}
