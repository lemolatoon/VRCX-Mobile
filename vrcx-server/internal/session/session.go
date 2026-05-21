// Package session manages server-side sessions backed by Postgres.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionDuration = 30 * 24 * time.Hour

// Session represents an authenticated user session.
type Session struct {
	ID            string
	UserID        string
	VRChatUserID  string
	CreatedAt     time.Time
	LastSeen      time.Time
	ExpiresAt     time.Time
}

// Store manages sessions in Postgres.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given Postgres pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create generates a new session for the given VRChat user.
func (s *Store) Create(ctx context.Context, vrchatUserID string) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID:           id,
		UserID:       vrchatUserID,
		VRChatUserID: vrchatUserID,
		CreatedAt:    time.Now(),
		LastSeen:     time.Now(),
		ExpiresAt:    time.Now().Add(sessionDuration),
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO sessions (id, user_id, vrchat_user_id, created_at, last_seen, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		sess.ID, sess.UserID, sess.VRChatUserID, sess.CreatedAt, sess.LastSeen, sess.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

// Get retrieves a session by ID, returning nil if not found or expired.
func (s *Store) Get(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, vrchat_user_id, created_at, last_seen, expires_at
		 FROM sessions WHERE id = $1 AND expires_at > NOW()`,
		id,
	).Scan(&sess.ID, &sess.UserID, &sess.VRChatUserID, &sess.CreatedAt, &sess.LastSeen, &sess.ExpiresAt)
	if err != nil {
		return nil, nil // not found or expired
	}
	// Touch last_seen async — fire-and-forget is fine here
	go func() {
		_, _ = s.db.Exec(context.Background(),
			`UPDATE sessions SET last_seen = NOW() WHERE id = $1`, id)
	}()
	return sess, nil
}

// Delete removes a session.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
