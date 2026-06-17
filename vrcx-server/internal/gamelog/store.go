// Package gamelog stores VRChat client log events uploaded by Windows agents.
package gamelog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

type Entry struct {
	LogFile    string         `json:"log_file"`
	LineOffset int64          `json:"line_offset"`
	CreatedAt  time.Time      `json:"created_at"`
	Type       string         `json:"type"`
	Payload    map[string]any `json:"payload"`
	RawLine    string         `json:"raw_line"`
}

type IngestRequest struct {
	SourceID string  `json:"source_id"`
	Entries  []Entry `json:"entries"`
}

type Cursor struct {
	CreatedAt time.Time
	ID        int64
}

type ListOpts struct {
	Types  []string
	Before *Cursor
	Limit  int
	Search string
}

type ListItem struct {
	ID        int64          `json:"id"`
	Type      string         `json:"type"`
	CreatedAt time.Time      `json:"created_at"`
	Payload   map[string]any `json:"payload"`
	RawLine   string         `json:"raw_line,omitempty"`
}

const defaultLimit = 50
const maxLimit = 200

func (s *Store) AppendBatch(ctx context.Context, viewerUserID, sourceID string, entries []Entry) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return fmt.Errorf("source_id required")
	}
	for _, e := range entries {
		if strings.TrimSpace(e.LogFile) == "" {
			return fmt.Errorf("log_file required")
		}
		if strings.TrimSpace(e.Type) == "" {
			return fmt.Errorf("type required")
		}
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now().UTC()
		}
		payload := e.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		_, err = s.db.Exec(ctx,
			`INSERT INTO gamelog_entries
			  (viewer_user_id, source_id, log_file, line_offset, created_at, type, payload, raw_line)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (viewer_user_id, source_id, log_file, line_offset) DO NOTHING`,
			viewerUserID, sourceID, e.LogFile, e.LineOffset, e.CreatedAt, e.Type, raw, e.RawLine,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) List(ctx context.Context, viewerUserID string, opts ListOpts) ([]ListItem, *Cursor, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	args := []any{viewerUserID}
	where := []string{"viewer_user_id=$1"}
	if len(opts.Types) > 0 {
		args = append(args, opts.Types)
		where = append(where, fmt.Sprintf("type = ANY($%d)", len(args)))
	}
	if opts.Before != nil {
		args = append(args, opts.Before.CreatedAt, opts.Before.ID)
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	if search := strings.TrimSpace(opts.Search); search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		where = append(where, fmt.Sprintf("(raw_line ILIKE $%d ESCAPE '\\' OR payload::text ILIKE $%d ESCAPE '\\')", len(args), len(args)))
	}
	args = append(args, limit+1)
	q := fmt.Sprintf(
		`SELECT id, type, created_at, payload, raw_line
		 FROM gamelog_entries
		 WHERE %s
		 ORDER BY created_at DESC, id DESC
		 LIMIT $%d`,
		strings.Join(where, " AND "), len(args),
	)
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]ListItem, 0, limit)
	for rows.Next() {
		var item ListItem
		var rawPayload []byte
		if err := rows.Scan(&item.ID, &item.Type, &item.CreatedAt, &rawPayload, &item.RawLine); err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(rawPayload, &item.Payload); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var next *Cursor
	if len(items) > limit {
		last := items[limit-1]
		next = &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		items = items[:limit]
	}
	return items, next, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}
