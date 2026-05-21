// Package store manages Postgres schema creation and migrations.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate ensures all required tables exist.
func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS allowed_users (
			vrchat_user_id TEXT PRIMARY KEY,
			note           TEXT NOT NULL DEFAULT '',
			added_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id             TEXT PRIMARY KEY,
			user_id        TEXT NOT NULL,
			vrchat_user_id TEXT NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at     TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id)`,
		`CREATE TABLE IF NOT EXISTS vrchat_credentials (
			vrchat_user_id TEXT PRIMARY KEY,
			cookies_enc    BYTEA NOT NULL,
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS rate_limit_attempts (
			key            TEXT NOT NULL,
			attempted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS rate_limit_attempts_key_idx ON rate_limit_attempts (key, attempted_at)`,
		// Future tables for collector (Phase 4) will be added here
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}
