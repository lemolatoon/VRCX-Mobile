// Package auth provides VRChat userId-based allowlist access control.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AllowlistEntry represents a permitted user.
type AllowlistEntry struct {
	VRChatUserID string
	Note         string
	AddedAt      time.Time
}

// Allowlist manages which VRChat user IDs may authenticate.
type Allowlist struct {
	db *pgxpool.Pool
}

// NewAllowlist creates an Allowlist backed by Postgres.
func NewAllowlist(db *pgxpool.Pool) *Allowlist {
	return &Allowlist{db: db}
}

// IsAllowed returns true if the given VRChat userId is in the allowlist.
func (a *Allowlist) IsAllowed(ctx context.Context, vrchatUserID string) (bool, error) {
	var exists bool
	err := a.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM allowed_users WHERE vrchat_user_id = $1)`,
		vrchatUserID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("allowlist check: %w", err)
	}
	return exists, nil
}

// Add inserts a new entry. Idempotent — updates note on conflict.
func (a *Allowlist) Add(ctx context.Context, vrchatUserID, note string) error {
	_, err := a.db.Exec(ctx,
		`INSERT INTO allowed_users (vrchat_user_id, note, added_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (vrchat_user_id) DO UPDATE SET note = EXCLUDED.note`,
		vrchatUserID, note,
	)
	return err
}

// Remove deletes an entry.
func (a *Allowlist) Remove(ctx context.Context, vrchatUserID string) error {
	_, err := a.db.Exec(ctx,
		`DELETE FROM allowed_users WHERE vrchat_user_id = $1`,
		vrchatUserID,
	)
	return err
}

// List returns all allowlisted users.
func (a *Allowlist) List(ctx context.Context) ([]AllowlistEntry, error) {
	rows, err := a.db.Query(ctx,
		`SELECT vrchat_user_id, note, added_at FROM allowed_users ORDER BY added_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AllowlistEntry
	for rows.Next() {
		var e AllowlistEntry
		if err := rows.Scan(&e.VRChatUserID, &e.Note, &e.AddedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
