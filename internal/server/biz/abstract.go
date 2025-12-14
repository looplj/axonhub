package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

type AbstractService struct {
	db *ent.Client
}

func (a *AbstractService) entFromContext(ctx context.Context) *ent.Client {
	db := ent.FromContext(ctx)
	if db != nil {
		return db
	}

	return a.db
}

func (a *AbstractService) RunInTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if tx := ent.TxFromContext(ctx); tx != nil {
		txClient := tx.Client()
		txCtx := ent.NewContext(ctx, txClient)

		return fn(txCtx)
	}

	db := a.entFromContext(ctx)

	tx, err := db.Tx(ctx)
	if err != nil {
		return err
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

	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	committed = true

	return nil
}
