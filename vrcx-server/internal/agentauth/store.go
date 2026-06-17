// Package agentauth manages bearer tokens used by Windows log agents.
package agentauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const tokenPrefix = "vrcxla_"

type Store struct {
	db *pgxpool.Pool
}

type Token struct {
	ID           string     `json:"id"`
	VRChatUserID string     `json:"-"`
	Name         string     `json:"name"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type CreatedToken struct {
	Token
	Plaintext string `json:"token"`
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, vrchatUserID, name string) (*CreatedToken, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Windows PC"
	}
	id, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	plain := tokenPrefix + base64.RawURLEncoding.EncodeToString(secret)
	hash := tokenHash(plain)
	now := time.Now().UTC()
	_, err = s.db.Exec(ctx,
		`INSERT INTO agent_tokens (id, vrchat_user_id, name, token_hash, created_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		id, vrchatUserID, name, hash[:], now,
	)
	if err != nil {
		return nil, fmt.Errorf("create agent token: %w", err)
	}
	return &CreatedToken{
		Token:     Token{ID: id, VRChatUserID: vrchatUserID, Name: name, CreatedAt: now},
		Plaintext: plain,
	}, nil
}

func (s *Store) List(ctx context.Context, vrchatUserID string) ([]Token, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, vrchat_user_id, name, created_at, last_used_at, revoked_at
		 FROM agent_tokens
		 WHERE vrchat_user_id=$1
		 ORDER BY created_at DESC`,
		vrchatUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.VRChatUserID, &t.Name, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Revoke(ctx context.Context, vrchatUserID, id string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE agent_tokens SET revoked_at=NOW()
		 WHERE id=$1 AND vrchat_user_id=$2 AND revoked_at IS NULL`,
		id, vrchatUserID,
	)
	return err
}

func (s *Store) Authenticate(ctx context.Context, bearer string) (string, error) {
	token := strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer "))
	if !strings.HasPrefix(token, tokenPrefix) {
		return "", fmt.Errorf("invalid token")
	}
	hash := tokenHash(token)
	var vrchatUserID string
	err := s.db.QueryRow(ctx,
		`UPDATE agent_tokens
		 SET last_used_at=NOW()
		 WHERE token_hash=$1 AND revoked_at IS NULL
		 RETURNING vrchat_user_id`,
		hash[:],
	).Scan(&vrchatUserID)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}
	return vrchatUserID, nil
}

func tokenHash(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
