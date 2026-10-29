// Package db embeds the SQL migrations so the API binary can apply and verify them.
package db

import "embed"

// Migrations holds the Goose SQL migrations in migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS
