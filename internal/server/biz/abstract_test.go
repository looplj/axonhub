package biz

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

func TestAbstractService_RunInTransaction(t *testing.T) {
	newSvc := func(t *testing.T) (*ent.Client, *AbstractService, context.Context) {
		client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
		svc := &AbstractService{db: client}
		ctx := privacy.DecisionContext(context.Background(), privacy.Allow)

		return client, svc, ctx
	}

	t.Run("commit", func(t *testing.T) {
		client, svc, ctx := newSvc(t)
		defer client.Close()

		var userID int

		err := svc.RunInTransaction(ctx, func(txCtx context.Context) error {
			created := ent.FromContext(txCtx).User.Create().
				SetEmail("test@example.com").
				SetPassword("password").
				SaveX(txCtx)
			userID = created.ID

			return nil
		})
		require.NoError(t, err)

		got := client.User.GetX(ctx, userID)
		require.Equal(t, userID, got.ID)
	})

	t.Run("rollback on error", func(t *testing.T) {
		client, svc, ctx := newSvc(t)
		defer client.Close()

		expectedErr := errors.New("boom")
		err := svc.RunInTransaction(ctx, func(txCtx context.Context) error {
			ent.FromContext(txCtx).User.Create().
				SetEmail("test@example.com").
				SetPassword("password").
				SaveX(txCtx)

			return expectedErr
		})
		require.ErrorIs(t, err, expectedErr)

		count := client.User.Query().CountX(ctx)
		require.Equal(t, 0, count)
	})

	t.Run("rollback on panic", func(t *testing.T) {
		client, svc, ctx := newSvc(t)
		defer client.Close()

		require.Panics(t, func() {
			_ = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
				ent.FromContext(txCtx).User.Create().
					SetEmail("test@example.com").
					SetPassword("password").
					SaveX(txCtx)
				panic("boom")
			})
		})

		count := client.User.Query().CountX(ctx)
		require.Equal(t, 0, count)
	})

	t.Run("existing tx context", func(t *testing.T) {
		client, svc, ctx := newSvc(t)
		defer client.Close()

		tx, err := client.Tx(ctx)
		require.NoError(t, err)

		txCtx := ent.NewTxContext(ctx, tx)
		err = svc.RunInTransaction(txCtx, func(txCtx context.Context) error {
			require.NotNil(t, ent.FromContext(txCtx))
			ent.FromContext(txCtx).User.Create().
				SetEmail("test@example.com").
				SetPassword("password").
				SaveX(txCtx)

			return nil
		})
		require.NoError(t, err)

		require.NoError(t, tx.Rollback())

		count := client.User.Query().CountX(ctx)
		require.Equal(t, 0, count)
	})

	t.Run("transactional client in context without Tx", func(t *testing.T) {
		// This tests the scenario where tx.Client() is stored in context
		// but TxFromContext returns nil (the Tx object itself is not stored).
		// This can happen when code passes the client through context but
		// forgets to also set NewTxContext.
		client, svc, ctx := newSvc(t)
		defer client.Close()

		tx, err := client.Tx(ctx)
		require.NoError(t, err)

		// Only store the transactional client, not the Tx itself
		txClient := tx.Client()
		txClientCtx := ent.NewContext(ctx, txClient)

		// This should NOT fail with "cannot start a transaction within a transaction"
		err = svc.RunInTransaction(txClientCtx, func(fnCtx context.Context) error {
			require.NotNil(t, ent.FromContext(fnCtx))
			ent.FromContext(fnCtx).User.Create().
				SetEmail("test@example.com").
				SetPassword("password").
				SaveX(fnCtx)

			return nil
		})
		require.NoError(t, err)

		require.NoError(t, tx.Commit())

		count := client.User.Query().CountX(ctx)
		require.Equal(t, 1, count)
	})

	t.Run("transactional client on service without context client", func(t *testing.T) {
		client, _, ctx := newSvc(t)
		defer client.Close()

		tx, err := client.Tx(ctx)
		require.NoError(t, err)

		txClient := tx.Client()
		svc := &AbstractService{db: txClient}

		err = svc.RunInTransaction(ctx, func(fnCtx context.Context) error {
			require.NotNil(t, ent.FromContext(fnCtx))
			ent.FromContext(fnCtx).User.Create().
				SetEmail("test@example.com").
				SetPassword("password").
				SaveX(fnCtx)

			return nil
		})
		require.NoError(t, err)

		require.NoError(t, tx.Commit())

		count := client.User.Query().CountX(ctx)
		require.Equal(t, 1, count)
	})
}
