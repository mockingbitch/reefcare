//go:build integration

package database_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

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
