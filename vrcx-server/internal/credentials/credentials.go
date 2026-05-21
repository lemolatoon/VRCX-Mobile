// Package credentials encrypts and persists per-user VRChat session cookies to Postgres.
// Uses AES-256-GCM. The encryption key must be 32 bytes, supplied as a base64 string
// via the COOKIE_ENCRYPTION_KEY environment variable.
package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// cookieRecord is a serialisable form of http.Cookie.
type cookieRecord struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	HttpOnly bool      `json:"httpOnly"`
	Secure   bool      `json:"secure"`
}

// Store encrypts and stores VRChat cookies per user.
type Store struct {
	db  *pgxpool.Pool
	key []byte // 32-byte AES-256 key
}

// New creates a Store. encryptionKeyB64 must be a base64-encoded 32-byte key.
func New(db *pgxpool.Pool, encryptionKeyB64 string) (*Store, error) {
	key, err := base64.StdEncoding.DecodeString(encryptionKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Store{db: db, key: key}, nil
}

// Save encrypts and upserts cookies for the given VRChat user ID.
func (s *Store) Save(ctx context.Context, vrchatUserID string, cookies []*http.Cookie) error {
	records := make([]cookieRecord, 0, len(cookies))
	for _, c := range cookies {
		records = append(records, cookieRecord{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Expires: c.Expires, HttpOnly: c.HttpOnly, Secure: c.Secure,
		})
	}
	plain, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal cookies: %w", err)
	}
	enc, err := s.encrypt(plain)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO vrchat_credentials (vrchat_user_id, cookies_enc, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (vrchat_user_id) DO UPDATE
		   SET cookies_enc = EXCLUDED.cookies_enc, updated_at = NOW()`,
		vrchatUserID, enc,
	)
	return err
}

// Load decrypts and returns cookies for the given VRChat user ID.
// Returns nil, nil if no record exists.
func (s *Store) Load(ctx context.Context, vrchatUserID string) ([]*http.Cookie, error) {
	var enc []byte
	err := s.db.QueryRow(ctx,
		`SELECT cookies_enc FROM vrchat_credentials WHERE vrchat_user_id = $1`,
		vrchatUserID,
	).Scan(&enc)
	if err != nil {
		return nil, nil // not found
	}
	plain, err := s.decrypt(enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var records []cookieRecord
	if err := json.Unmarshal(plain, &records); err != nil {
		return nil, fmt.Errorf("unmarshal cookies: %w", err)
	}
	cookies := make([]*http.Cookie, 0, len(records))
	for _, r := range records {
		cookies = append(cookies, &http.Cookie{
			Name: r.Name, Value: r.Value, Domain: r.Domain,
			Path: r.Path, Expires: r.Expires, HttpOnly: r.HttpOnly, Secure: r.Secure,
		})
	}
	return cookies, nil
}

// LoadAll returns all VRChat user IDs that have stored credentials.
func (s *Store) LoadAll(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT vrchat_user_id FROM vrchat_credentials`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) encrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (s *Store) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
