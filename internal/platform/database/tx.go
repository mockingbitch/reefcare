package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TxStarter begins transactions. *pgxpool.Pool starts a real transaction; pgx.Tx starts
// a savepoint, so a service step can run inside a transaction its caller already opened.
type TxStarter interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// InTx runs fn inside a read-committed transaction. The transaction commits when fn
// returns nil and rolls back otherwise (including on panic). It is the only way
// application services start transactions, so boundaries stay explicit:
//
//	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
//		q := dbq.New(tx)
//		...
//	})
func InTx(ctx context.Context, db TxStarter, fn func(tx pgx.Tx) error) error {
	return inTx(ctx, db, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

// InSerializableTx is InTx with SERIALIZABLE isolation for workflows that read and
// then write based on what they read. It does not retry: callers must handle
// serialization failures (SQLSTATE 40001) and keep fn free of external side effects.
func InSerializableTx(ctx context.Context, db TxStarter, fn func(tx pgx.Tx) error) error {
	return inTx(ctx, db, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
}

func inTx(ctx context.Context, db TxStarter, opts pgx.TxOptions, fn func(tx pgx.Tx) error) error {
	if err := pgx.BeginTxFunc(ctx, db, opts, fn); err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	return nil
}
