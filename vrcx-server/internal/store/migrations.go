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

		// Phase 4: feed tables (mirroring VRCX feed_* schema, co-tenanted by viewer_user_id)
		`CREATE TABLE IF NOT EXISTS feed_gps (
			id               BIGSERIAL PRIMARY KEY,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			viewer_user_id   TEXT NOT NULL,
			vrchat_user_id   TEXT NOT NULL,
			display_name     TEXT NOT NULL,
			location         TEXT NOT NULL,
			previous_location TEXT NOT NULL,
			world_name       TEXT,
			group_name       TEXT,
			time_ms          BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS feed_gps_viewer_created_idx ON feed_gps (viewer_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS feed_status (
			id                          BIGSERIAL PRIMARY KEY,
			created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			viewer_user_id              TEXT NOT NULL,
			vrchat_user_id              TEXT NOT NULL,
			display_name                TEXT NOT NULL,
			status                      TEXT NOT NULL,
			previous_status             TEXT NOT NULL,
			status_description          TEXT,
			previous_status_description TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS feed_status_viewer_created_idx ON feed_status (viewer_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS feed_bio (
			id               BIGSERIAL PRIMARY KEY,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			viewer_user_id   TEXT NOT NULL,
			vrchat_user_id   TEXT NOT NULL,
			display_name     TEXT NOT NULL,
			bio              TEXT NOT NULL,
			previous_bio     TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS feed_bio_viewer_created_idx ON feed_bio (viewer_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS feed_avatar (
			id                                          BIGSERIAL PRIMARY KEY,
			created_at                                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			viewer_user_id                              TEXT NOT NULL,
			vrchat_user_id                              TEXT NOT NULL,
			display_name                                TEXT NOT NULL,
			owner_id                                    TEXT,
			avatar_name                                 TEXT,
			current_avatar_image_url                    TEXT NOT NULL,
			current_avatar_thumbnail_image_url          TEXT NOT NULL,
			previous_current_avatar_image_url           TEXT NOT NULL,
			previous_current_avatar_thumbnail_image_url TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS feed_avatar_viewer_created_idx ON feed_avatar (viewer_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS feed_online_offline (
			id               BIGSERIAL PRIMARY KEY,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			viewer_user_id   TEXT NOT NULL,
			vrchat_user_id   TEXT NOT NULL,
			display_name     TEXT NOT NULL,
			type             TEXT NOT NULL,
			location         TEXT,
			world_name       TEXT,
			group_name       TEXT,
			time_ms          BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS feed_online_offline_viewer_created_idx ON feed_online_offline (viewer_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS cached_users (
			viewer_user_id TEXT NOT NULL,
			vrchat_user_id TEXT NOT NULL,
			snapshot_json  JSONB NOT NULL,
			state          TEXT NOT NULL,
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (viewer_user_id, vrchat_user_id)
		)`,

		`CREATE TABLE IF NOT EXISTS agent_tokens (
			id             TEXT PRIMARY KEY,
			vrchat_user_id TEXT NOT NULL,
			name           TEXT NOT NULL,
			token_hash     BYTEA NOT NULL UNIQUE,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at   TIMESTAMPTZ,
			revoked_at     TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS agent_tokens_user_idx ON agent_tokens (vrchat_user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS gamelog_entries (
			id             BIGSERIAL PRIMARY KEY,
			viewer_user_id TEXT NOT NULL,
			source_id      TEXT NOT NULL,
			log_file       TEXT NOT NULL,
			line_offset    BIGINT NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL,
			type           TEXT NOT NULL,
			payload        JSONB NOT NULL,
			raw_line       TEXT NOT NULL DEFAULT '',
			ingested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(viewer_user_id, source_id, log_file, line_offset)
		)`,
		`CREATE INDEX IF NOT EXISTS gamelog_entries_viewer_created_idx ON gamelog_entries (viewer_user_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS gamelog_entries_viewer_type_created_idx ON gamelog_entries (viewer_user_id, type, created_at DESC)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}
