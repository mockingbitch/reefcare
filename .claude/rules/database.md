---
paths:
  - "db/**"
  - "**/*.sql"
  - "sqlc.yaml"
  - "sqlc.yml"
  - "docs/database-design.md"
---

# PostgreSQL, migrations and sqlc

## Schema
- PostgreSQL is the source of truth. Invariants are enforced by the database (PK, FK, UNIQUE,
  CHECK, NOT NULL, exclusion constraints), not only in Go.
- UUID primary keys for public-facing entities (`uuid` type, generated server-side). Document any
  exception.
- Every FK states an explicit `ON DELETE` policy. Default to `RESTRICT` for historical records;
  `CASCADE` only for owned child rows with no historical value.
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` and `updated_at` on mutable tables.
- Event times are `TIMESTAMPTZ` (stored UTC). Distinguish `measured_at` vs `recorded_at`,
  `scheduled_for` vs `executed_at`.
- Measurements and dosing amounts: `NUMERIC(p,s)` + explicit unit column; keep original value/unit.
- JSONB only for flexible metadata, external device payloads, or versioned assessment / case
  snapshots. Never for data that is filtered, joined, or constrained relationally.
- Soft delete only with a documented business reason. Historical records (water tests, dosing
  events, livestock events, reputation ledger, audit logs) are never hard-deleted by normal flows.
- Use a `version` column (optimistic concurrency) where concurrent edits could lose data.
- Idempotency: unique constraints on idempotency keys, `(job_type, dedupe_key)` for scheduled jobs,
  and `(user_id, source_type, source_id, event_type)` style keys for reputation rewards.
- Votes: `UNIQUE (voter_id, target_type, target_id)` + CHECK preventing self-votes where possible.
- Index for real query patterns, e.g. `(tank_id, parameter_type_id, measured_at DESC)`; composite
  indexes match filter + order. Partitioning only when data volume justifies it.

## Migrations (Goose)
- Every schema change is a new migration in `db/migrations/` — never edit a migration that may
  have been applied anywhere; add a new one.
- Each migration has a working `-- +goose Down` or an explicit comment explaining why it is
  irreversible.
- Data-destructive or locking changes (dropping columns, rewriting large tables, non-concurrent
  index builds on big tables) must be called out in the summary with a rollout strategy.
- Deterministic seed data for dev/test lives separately from schema migrations.

## sqlc
- Queries live in `db/queries/<module>.sql`, named `-- name: VerbNoun :one|:many|:exec|:execrows`.
- Select only needed columns; no `SELECT *` in application queries.
- Collections use keyset (cursor) pagination ordered by a unique, indexed key.
- Avoid N+1: batch with `= ANY($1::uuid[])` or joins.
- Regenerate with `sqlc generate` after any query/schema change; never hand-edit generated code.
- Every tank-owned query filters by an authorized tank/owner scope, or is only callable after an
  explicit authorization check in the service.
- Inspect `EXPLAIN (ANALYZE, BUFFERS)` for important list/trend/timeline queries.
