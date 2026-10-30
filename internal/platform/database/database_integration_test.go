//go:build integration

package database_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"reefcare/db"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/database/dbtest"
)

// TestMigrationsUpDownUp proves every embedded migration fully reverses and re-applies.
// dbtest.New provides a fresh database that is already migrated up.
func TestMigrationsUpDownUp(t *testing.T) {
	pool := dbtest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	m, err := database.NewMigrator(pool, db.Migrations, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	if err := m.CheckCompatible(ctx); err != nil {
		t.Fatalf("after up: %v", err)
	}
	statuses, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for range statuses {
		if err := m.Down(ctx); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	var tables int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("%d tables left after rolling back every migration", tables)
	}
	if err := m.CheckCompatible(ctx); err == nil {
		t.Fatal("expected schema to be behind after rolling back")
	}
	if err := m.Up(ctx); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if err := m.CheckCompatible(ctx); err != nil {
		t.Fatalf("after second up: %v", err)
	}
}

// Two transactions that each read what the other writes cannot both commit under
// SERIALIZABLE; the loser is re-run and then succeeds.
func TestInSerializableTxRetries(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE TABLE ledger (k text PRIMARY KEY, v int NOT NULL); INSERT INTO ledger VALUES ('a', 0), ('b', 0)`); err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	ready := make(chan struct{})
	var once sync.Once
	run := func(read, write string) error {
		return database.InSerializableTx(ctx, pool, func(tx pgx.Tx) error {
			n := attempts.Add(1)
			var sum int
			if err := tx.QueryRow(ctx, `SELECT sum(v) FROM ledger WHERE k = $1`, read).Scan(&sum); err != nil {
				return err
			}
			if n <= 2 { // make the first two attempts overlap
				once.Do(func() { close(ready) })
				<-ready
				time.Sleep(50 * time.Millisecond)
			}
			_, err := tx.Exec(ctx, `UPDATE ledger SET v = v + 1 WHERE k = $1`, write)
			return err
		})
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Go(func() { errs[0] = run("a", "b") })
	wg.Go(func() { errs[1] = run("b", "a") })
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("transaction failed despite retry: %v", err)
		}
	}
	if attempts.Load() < 3 {
		t.Fatalf("attempts = %d, want a retry", attempts.Load())
	}
	var total int
	if err := pool.QueryRow(ctx, `SELECT sum(v) FROM ledger`).Scan(&total); err != nil || total != 2 {
		t.Fatalf("total = %d, err %v", total, err)
	}
}
