-- +goose Up
-- citext: case-insensitive email and username columns (see docs/database-design.md §3).
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose Down
DROP EXTENSION IF EXISTS citext;
