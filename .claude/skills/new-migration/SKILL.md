---
name: new-migration
description: Create a new Goose SQL migration for ReefCare with constraints, indexes and a reversible Down, then regenerate sqlc code. Use whenever the database schema must change.
argument-hint: "<short_snake_case_description>"
---

Create a migration for: **$ARGUMENTS**

1. Read `.claude/rules/database.md`, the latest migrations in `db/migrations/`, and
   `docs/database-design.md`.
2. Never edit an existing committed migration — always add a new one.
3. Create the file: `goose -dir db/migrations create $ARGUMENTS sql`
   (if goose is not installed, create `db/migrations/<YYYYMMDDHHMMSS>_$ARGUMENTS.sql` manually with
   a UTC timestamp and say so).
4. Write `-- +goose Up` with:
   - UUID PKs, `TIMESTAMPTZ` timestamps, `NUMERIC` + unit for measurements/doses.
   - FKs with explicit `ON DELETE` policy; historical tables use `RESTRICT`.
   - NOT NULL / CHECK / UNIQUE constraints for every invariant (idempotency keys, dedupe keys,
     one-vote-per-user, no self-vote, valid states).
   - Indexes for the planned query patterns (filter columns then order column).
   - `-- +goose NO TRANSACTION` only when using `CREATE INDEX CONCURRENTLY`.
5. Write `-- +goose Down` that fully reverses Up, or a comment explaining why it cannot.
6. Add or update queries in `db/queries/<module>.sql`, then run `sqlc generate`.
7. If a local PostgreSQL is available, verify `goose up` → `goose down` → `goose up` on a
   disposable database.
8. Update `docs/database-design.md` (table purpose, relationships, constraints, ER diagram).
9. In the summary, call out locking, data rewrite, or data-loss risks and the rollout strategy.
