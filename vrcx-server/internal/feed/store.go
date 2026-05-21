package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles feed entry persistence and user snapshot caching.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// AppendBatch writes all entries returned by Diff into the appropriate tables.
func (s *Store) AppendBatch(ctx context.Context, entries []any) error {
	for _, e := range entries {
		if err := s.appendOne(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) appendOne(ctx context.Context, e any) error {
	switch v := e.(type) {
	case GPSEntry:
		_, err := s.db.Exec(ctx,
			`INSERT INTO feed_gps (viewer_user_id, vrchat_user_id, display_name, location, previous_location, world_name, group_name, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			v.ViewerUserID, v.VRChatUserID, v.DisplayName, v.Location, v.PreviousLocation,
			v.WorldName, v.GroupName, v.CreatedAt,
		)
		return err

	case StatusEntry:
		_, err := s.db.Exec(ctx,
			`INSERT INTO feed_status (viewer_user_id, vrchat_user_id, display_name, status, previous_status, status_description, previous_status_description, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			v.ViewerUserID, v.VRChatUserID, v.DisplayName, v.Status, v.PreviousStatus,
			v.StatusDescription, v.PreviousStatusDescription, v.CreatedAt,
		)
		return err

	case BioEntry:
		_, err := s.db.Exec(ctx,
			`INSERT INTO feed_bio (viewer_user_id, vrchat_user_id, display_name, bio, previous_bio, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			v.ViewerUserID, v.VRChatUserID, v.DisplayName, v.Bio, v.PreviousBio, v.CreatedAt,
		)
		return err

	case AvatarEntry:
		_, err := s.db.Exec(ctx,
			`INSERT INTO feed_avatar (viewer_user_id, vrchat_user_id, display_name, owner_id, avatar_name,
			  current_avatar_image_url, current_avatar_thumbnail_image_url,
			  previous_current_avatar_image_url, previous_current_avatar_thumbnail_image_url, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			v.ViewerUserID, v.VRChatUserID, v.DisplayName, v.OwnerID, v.AvatarName,
			v.CurrentAvatarImageURL, v.CurrentAvatarThumbnailImageURL,
			v.PreviousCurrentAvatarImageURL, v.PreviousCurrentAvatarThumbnailImageURL,
			v.CreatedAt,
		)
		return err

	case OnlineOfflineEntry:
		_, err := s.db.Exec(ctx,
			`INSERT INTO feed_online_offline (viewer_user_id, vrchat_user_id, display_name, type, location, world_name, group_name, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			v.ViewerUserID, v.VRChatUserID, v.DisplayName, v.Type, v.Location,
			v.WorldName, v.GroupName, v.CreatedAt,
		)
		return err

	default:
		return fmt.Errorf("feed.Store.appendOne: unknown entry type %T", e)
	}
}

// LoadCache retrieves the last known snapshot for a friend.
// Returns nil, "", nil if no entry exists yet.
func (s *Store) LoadCache(ctx context.Context, viewerUserID, vrchatUserID string) (map[string]any, string, error) {
	var raw []byte
	var state string
	err := s.db.QueryRow(ctx,
		`SELECT snapshot_json, state FROM cached_users WHERE viewer_user_id=$1 AND vrchat_user_id=$2`,
		viewerUserID, vrchatUserID,
	).Scan(&raw, &state)
	if err != nil {
		// pgx returns pgx.ErrNoRows — treat as empty cache
		return nil, "", nil
	}
	var snap map[string]any
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, "", fmt.Errorf("feed.LoadCache: unmarshal: %w", err)
	}
	return snap, state, nil
}

// SaveCache upserts the latest snapshot for a friend.
func (s *Store) SaveCache(ctx context.Context, viewerUserID, vrchatUserID string, snap map[string]any, state string) error {
	raw, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("feed.SaveCache: marshal: %w", err)
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO cached_users (viewer_user_id, vrchat_user_id, snapshot_json, state, updated_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (viewer_user_id, vrchat_user_id) DO UPDATE
		   SET snapshot_json=$3, state=$4, updated_at=$5`,
		viewerUserID, vrchatUserID, raw, state, time.Now(),
	)
	return err
}

// --- List / pagination ---

// Cursor is an opaque keyset pagination cursor.
type Cursor struct {
	CreatedAt time.Time
	ID        int64
}

// ListOpts configures a feed list query.
type ListOpts struct {
	Types  []string // empty = all; valid: GPS Status Bio Avatar Online Offline
	Before *Cursor  // exclusive upper bound
	Limit  int      // 0 = default (50)
}

// ListItem is a single de-typed feed entry.
type ListItem struct {
	ID        int64
	Type      string
	CreatedAt time.Time
	Payload   map[string]any
}

const defaultLimit = 50
const maxLimit = 200

// List returns feed entries for a viewer, newest-first, with cursor pagination.
func (s *Store) List(ctx context.Context, viewerUserID string, opts ListOpts) ([]ListItem, *Cursor, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	// Build UNION ALL across all five tables.
	// Each sub-query returns: id, type_label, created_at, and a JSON payload.
	const (
		qGPS = `
		SELECT id, 'GPS' AS type, created_at,
		  jsonb_build_object('vrchatUserId',vrchat_user_id,'displayName',display_name,
		    'location',location,'previousLocation',previous_location,
		    'worldName',world_name,'groupName',group_name) AS payload
		FROM feed_gps WHERE viewer_user_id=$1`

		qStatus = `
		SELECT id, 'Status' AS type, created_at,
		  jsonb_build_object('vrchatUserId',vrchat_user_id,'displayName',display_name,
		    'status',status,'previousStatus',previous_status,
		    'statusDescription',status_description,'previousStatusDescription',previous_status_description) AS payload
		FROM feed_status WHERE viewer_user_id=$1`

		qBio = `
		SELECT id, 'Bio' AS type, created_at,
		  jsonb_build_object('vrchatUserId',vrchat_user_id,'displayName',display_name,
		    'bio',bio,'previousBio',previous_bio) AS payload
		FROM feed_bio WHERE viewer_user_id=$1`

		qAvatar = `
		SELECT id, 'Avatar' AS type, created_at,
		  jsonb_build_object('vrchatUserId',vrchat_user_id,'displayName',display_name,
		    'ownerId',owner_id,'avatarName',avatar_name,
		    'currentAvatarImageUrl',current_avatar_image_url,
		    'currentAvatarThumbnailImageUrl',current_avatar_thumbnail_image_url,
		    'previousCurrentAvatarImageUrl',previous_current_avatar_image_url,
		    'previousCurrentAvatarThumbnailImageUrl',previous_current_avatar_thumbnail_image_url) AS payload
		FROM feed_avatar WHERE viewer_user_id=$1`

		qOnOff = `
		SELECT id, type, created_at,
		  jsonb_build_object('vrchatUserId',vrchat_user_id,'displayName',display_name,
		    'type',type,'location',location,'worldName',world_name,'groupName',group_name) AS payload
		FROM feed_online_offline WHERE viewer_user_id=$1`
	)

	// Type filter: build allowed set
	allowed := map[string]bool{}
	if len(opts.Types) > 0 {
		for _, t := range opts.Types {
			allowed[t] = true
		}
	}

	var parts []string
	for _, q := range []struct {
		label string
		q     string
	}{
		{"GPS", qGPS}, {"Status", qStatus}, {"Bio", qBio},
		{"Avatar", qAvatar}, {"Online", qOnOff}, {"Offline", qOnOff},
	} {
		if len(allowed) == 0 || allowed[q.label] {
			// Online and Offline both use qOnOff; the type column from the table
			// itself carries the exact label, so we can include both selects.
			if q.label != "Offline" { // de-dup: Online select already covers both
				parts = append(parts, q.q)
			}
		}
	}
	// Edge case: if only Offline requested, still need qOnOff
	if len(allowed) > 0 && allowed["Offline"] && !allowed["Online"] {
		parts = append(parts, qOnOff)
	}

	if len(parts) == 0 {
		return nil, nil, nil
	}

	union := ""
	for i, p := range parts {
		if i > 0 {
			union += " UNION ALL "
		}
		union += p
	}

	var cursorClause string
	if opts.Before != nil {
		cursorClause = fmt.Sprintf(" AND (created_at, id) < ('%s', %d)", opts.Before.CreatedAt.UTC().Format(time.RFC3339Nano), opts.Before.ID)
	}

	// Wrap the UNION with viewer_user_id filter already embedded via $1.
	// Add cursor filter and ordering on the outer query.
	query := fmt.Sprintf(
		`SELECT id, type, created_at, payload FROM (%s) sub WHERE 1=1%s ORDER BY created_at DESC, id DESC LIMIT %d`,
		union, cursorClause, limit+1,
	)

	rows, err := s.db.Query(ctx, query, viewerUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("feed.List: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var it ListItem
		var payloadJSON []byte
		if err := rows.Scan(&it.ID, &it.Type, &it.CreatedAt, &payloadJSON); err != nil {
			return nil, nil, fmt.Errorf("feed.List scan: %w", err)
		}
		if err := json.Unmarshal(payloadJSON, &it.Payload); err != nil {
			return nil, nil, fmt.Errorf("feed.List unmarshal payload: %w", err)
		}
		items = append(items, it)
	}
	if rows.Err() != nil {
		return nil, nil, fmt.Errorf("feed.List rows: %w", rows.Err())
	}

	var nextCursor *Cursor
	if len(items) > limit {
		last := items[limit-1]
		nextCursor = &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		items = items[:limit]
	}

	return items, nextCursor, nil
}
