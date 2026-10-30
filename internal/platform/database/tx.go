package database

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

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

// SerializableAttempts bounds how often InSerializableTx runs fn.
const SerializableAttempts = 3

// InSerializableTx is InTx with SERIALIZABLE isolation for workflows that read and
// then write based on what they read. A serialization failure or deadlock (SQLSTATE
// 40001/40P01) re-runs the whole transaction up to SerializableAttempts times with a
// short jittered backoff, so fn must be free of external side effects (send emails
// after commit). Nested calls (db is a pgx.Tx) never retry: the outer transaction is
// already aborted and only its owner can restart it.
func InSerializableTx(ctx context.Context, db TxStarter, fn func(tx pgx.Tx) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if _, nested := db.(pgx.Tx); nested {
		return inTx(ctx, db, opts, fn)
	}
	var err error
	for attempt := 1; ; attempt++ {
		err = inTx(ctx, db, opts, fn)
		if err == nil || !IsSerializationFailure(err) || attempt == SerializableAttempts {
			return err
		}
		backoff := time.Duration(attempt)*10*time.Millisecond + time.Duration(rand.Int64N(int64(10*time.Millisecond))) //nolint:gosec // jitter, not security
		select {
		case <-ctx.Done():
			return err
		case <-time.After(backoff):
		}
	}
}

func inTx(ctx context.Context, db TxStarter, opts pgx.TxOptions, fn func(tx pgx.Tx) error) error {
	if err := pgx.BeginTxFunc(ctx, db, opts, fn); err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	return nil
}
