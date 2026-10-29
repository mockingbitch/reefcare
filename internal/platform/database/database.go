// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// PoolConfig configures the connection pool.
type PoolConfig struct {
	URL            string
	MaxConns       int32
	ConnectTimeout time.Duration
}

// Open creates a pgx pool and verifies connectivity.
func Open(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// The parse error may echo the URL including the password; don't wrap it.
		return nil, fmt.Errorf("parse database url: invalid format")
	}
	pc.MaxConns = cfg.MaxConns
	pc.MaxConnIdleTime = 5 * time.Minute
	pc.MaxConnLifetime = time.Hour
	pc.HealthCheckPeriod = 30 * time.Second
	pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	pc.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pc.ConnConfig.RuntimeParams["application_name"] = "reefcare-api"

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrator applies and inspects Goose migrations from an embedded filesystem.
type Migrator struct {
	db       *sql.DB
	provider *goose.Provider
}

// NewMigrator builds a migrator over migrations in fsys (the root holding migrations/).
// A Postgres advisory lock prevents concurrent instances from migrating at the same time.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS, log *slog.Logger) (*Migrator, error) {
	sub, err := fs.Sub(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open migrations dir: %w", err)
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration locker: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, db, sub,
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(log),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Migrator{db: db, provider: p}, nil
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	_, err := m.provider.Up(ctx)
	return err
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context) error {
	_, err := m.provider.Down(ctx)
	return err
}

// Status returns the state of each known migration.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return m.provider.Status(ctx)
}

// LatestVersion is the highest migration version embedded in the binary.
func (m *Migrator) LatestVersion() int64 {
	var latest int64
	for _, s := range m.provider.ListSources() {
		latest = max(latest, s.Version)
	}
	return latest
}

// CheckCompatible returns an error when the database schema is older than the latest
// migration embedded in the binary. A newer schema is accepted, so an old instance stays
// ready while a deploy migrates ahead of it and after a code-only rollback; migrations
// must therefore follow expand/contract (backward-compatible) rules.
func (m *Migrator) CheckCompatible(ctx context.Context) error {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if want := m.LatestVersion(); v < want {
		return fmt.Errorf("schema version %d is behind binary's %d", v, want)
	}
	return nil
}

// Close releases the sql.DB wrapper. It does not close the underlying pool.
func (m *Migrator) Close() error { return m.db.Close() }
