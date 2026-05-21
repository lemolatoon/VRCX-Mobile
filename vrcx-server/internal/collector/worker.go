// Package collector implements per-user VRChat WebSocket goroutines.
package collector

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/credentials"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/feed"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/vrcapi"
)

// Worker maintains a persistent VRChat pipeline WebSocket for one user.
type Worker struct {
	vrchatUserID string
	creds        *credentials.Store
	feedStore    *feed.Store
	log          *slog.Logger
}

func newWorker(vrchatUserID string, creds *credentials.Store, feedStore *feed.Store) *Worker {
	return &Worker{
		vrchatUserID: vrchatUserID,
		creds:        creds,
		feedStore:    feedStore,
		log:          slog.With("vrchatUserID", vrchatUserID),
	}
}

// Run loops forever (connecting, reading, reconnecting) until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	backoff := 5 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			w.log.Warn("pipeline disconnected; reconnecting", "err", err, "in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			// exponential backoff with jitter, capped at 5 min
			backoff = min(backoff*2+jitter(), 5*time.Minute)
		} else {
			backoff = 5 * time.Second
		}
	}
}

func jitter() time.Duration {
	return time.Duration(rand.N(int64(2 * time.Second)))
}

func (w *Worker) runOnce(ctx context.Context) error {
	cookies, err := w.creds.Load(ctx, w.vrchatUserID)
	if err != nil || len(cookies) == 0 {
		return errors.New("no credentials found")
	}

	client, err := vrcapi.NewClient()
	if err != nil {
		return err
	}
	client.SetCookies(cookies)

	// Seed cached_users with current friend state before opening WS.
	// This mirrors VRCX's initial friend load and ensures every friend has a
	// diff baseline so the first WS event can generate feed entries correctly.
	if err := w.seedFriendCache(ctx, client); err != nil {
		// Non-fatal: log and continue — WS events will seed lazily.
		w.log.Warn("seedFriendCache", "err", err)
	}

	token, err := client.FetchPipelineToken(ctx)
	if err != nil {
		return err
	}

	conn, err := vrcapi.DialPipeline(ctx, token)
	if err != nil {
		return err
	}
	defer conn.Close()

	w.log.Info("pipeline connected")

	for {
		evt, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := w.handleEvent(ctx, evt); err != nil {
			w.log.Warn("handleEvent", "err", err)
		}
	}
}

// seedFriendCache loads all friends from the VRChat API and writes them into
// cached_users only when no entry exists yet (so existing cache is not clobbered).
func (w *Worker) seedFriendCache(ctx context.Context, client *vrcapi.Client) error {
	friends, err := client.FetchAllFriends(ctx)
	if err != nil {
		return err
	}
	seeded := 0
	for _, f := range friends {
		existing, _, err := w.feedStore.LoadCache(ctx, w.vrchatUserID, f.ID)
		if err != nil || existing != nil {
			continue // already have a baseline
		}
		state := f.State
		if state == "" {
			if f.Location == "offline" || f.Location == "" {
				state = "offline"
			} else {
				state = "online"
			}
		}
		snap := map[string]any{
			"id":                             f.ID,
			"displayName":                    f.DisplayName,
			"state":                          state,
			"status":                         f.Status,
			"statusDescription":              f.StatusDescription,
			"bio":                            f.Bio,
			"location":                       f.Location,
			"currentAvatarImageUrl":          f.CurrentAvatarImageURL,
			"currentAvatarThumbnailImageUrl": f.CurrentAvatarThumbnailImageURL,
		}
		if err := w.feedStore.SaveCache(ctx, w.vrchatUserID, f.ID, snap, state); err != nil {
			w.log.Warn("seedFriendCache: save", "friend", f.ID, "err", err)
		} else {
			seeded++
		}
	}
	w.log.Info("seedFriendCache done", "seeded", seeded, "total", len(friends))
	return nil
}

// pipelineUserPayload is the shape of content.user in most pipeline events.
type pipelineUserPayload struct {
	ID                             string `json:"id"`
	DisplayName                    string `json:"displayName"`
	State                          string `json:"state"`
	Status                         string `json:"status"`
	StatusDescription              string `json:"statusDescription"`
	Bio                            string `json:"bio"`
	Location                       string `json:"location"`
	WorldID                        string `json:"worldId"`
	TravelingToLocation            string `json:"travelingToLocation"`
	CurrentAvatarImageURL          string `json:"currentAvatarImageUrl"`
	CurrentAvatarThumbnailImageURL string `json:"currentAvatarThumbnailImageUrl"`
}

func (w *Worker) handleEvent(ctx context.Context, evt vrcapi.PipelineEvent) error {
	var nextSnap map[string]any
	nextState := ""

	switch evt.Type {
	case "friend-online":
		var content struct {
			UserID              string              `json:"userId"`
			Platform            string              `json:"platform"`
			Location            string              `json:"location"`
			WorldID             string              `json:"worldId"`
			TravelingToLocation string              `json:"travelingToLocation"`
			User                pipelineUserPayload `json:"user"`
		}
		if err := json.Unmarshal(evt.Content, &content); err != nil {
			return err
		}
		nextSnap = userToSnap(content.User)
		nextSnap["state"] = "online"
		nextSnap["location"] = coalesce(content.Location, nextSnap["location"])
		nextState = "online"

	case "friend-active":
		var content struct {
			UserID   string              `json:"userId"`
			Platform string              `json:"platform"`
			User     pipelineUserPayload `json:"user"`
		}
		if err := json.Unmarshal(evt.Content, &content); err != nil {
			return err
		}
		nextSnap = userToSnap(content.User)
		nextSnap["state"] = "active"
		nextSnap["location"] = "offline"
		nextState = "active"

	case "friend-offline":
		var content struct {
			UserID string `json:"userId"`
		}
		if err := json.Unmarshal(evt.Content, &content); err != nil {
			return err
		}
		prevSnap, _, _ := w.feedStore.LoadCache(ctx, w.vrchatUserID, content.UserID)
		if prevSnap == nil {
			prevSnap = map[string]any{"id": content.UserID, "displayName": content.UserID}
		}
		nextSnap = copySnap(prevSnap)
		nextSnap["state"] = "offline"
		nextSnap["location"] = "offline"
		nextState = "offline"

	case "friend-update":
		var content struct {
			UserID string              `json:"userId"`
			User   pipelineUserPayload `json:"user"`
		}
		if err := json.Unmarshal(evt.Content, &content); err != nil {
			return err
		}
		nextSnap = userToSnap(content.User)
		if s, _ := nextSnap["state"].(string); s != "" {
			nextState = s
		} else {
			_, cachedState, _ := w.feedStore.LoadCache(ctx, w.vrchatUserID, content.User.ID)
			nextState = coalesce(cachedState, "online")
			nextSnap["state"] = nextState
		}

	case "friend-location":
		var content struct {
			UserID              string              `json:"userId"`
			Location            string              `json:"location"`
			WorldID             string              `json:"worldId"`
			TravelingToLocation string              `json:"travelingToLocation"`
			User                pipelineUserPayload `json:"user"`
		}
		if err := json.Unmarshal(evt.Content, &content); err != nil {
			return err
		}
		nextSnap = userToSnap(content.User)
		nextSnap["state"] = "online"
		nextSnap["location"] = coalesce(content.Location, nextSnap["location"])
		nextState = "online"

		// Traveling: preserve the pre-travel location in the snapshot so that
		// when the next friend-location arrives with the destination, GPS can
		// show "source → destination" rather than "traveling → destination".
		// This mirrors VRCX's $previousLocation tracking.
		if coalesce(content.Location, "") == "traveling" {
			prevSnap, _, _ := w.feedStore.LoadCache(ctx, w.vrchatUserID, coalesce(content.UserID, content.User.ID))
			if prevSnap != nil {
				prevLoc, _ := prevSnap["location"].(string)
				if prevLoc != "" && prevLoc != "traveling" && prevLoc != "offline" {
					nextSnap["$previousLocation"] = prevLoc
				}
			}
		}

	default:
		return nil
	}

	if nextSnap == nil {
		return nil
	}

	vrcID, _ := nextSnap["id"].(string)
	if vrcID == "" {
		return nil
	}

	prevSnap, prevState, err := w.feedStore.LoadCache(ctx, w.vrchatUserID, vrcID)
	if err != nil {
		return err
	}

	if prevSnap == nil {
		// No baseline yet (seedFriendCache missed this user) — seed silently.
		return w.feedStore.SaveCache(ctx, w.vrchatUserID, vrcID, nextSnap, nextState)
	}

	// Resolve traveling: if user was marked as traveling, restore the pre-travel
	// location so GPS shows the real source world.
	if loc, _ := nextSnap["location"].(string); loc != "" && loc != "traveling" {
		if prevLoc, _ := prevSnap["location"].(string); prevLoc == "traveling" {
			if saved, ok := prevSnap["$previousLocation"].(string); ok && saved != "" {
				prevSnap["location"] = saved
			}
		}
	}

	entries := feed.Diff(w.vrchatUserID, prevSnap, prevState, nextSnap, nextState, time.Now())
	if len(entries) > 0 {
		if err := w.feedStore.AppendBatch(ctx, entries); err != nil {
			w.log.Warn("AppendBatch", "err", err)
		}
	}

	return w.feedStore.SaveCache(ctx, w.vrchatUserID, vrcID, nextSnap, nextState)
}

func userToSnap(u pipelineUserPayload) map[string]any {
	return map[string]any{
		"id":                             u.ID,
		"displayName":                    u.DisplayName,
		"state":                          u.State,
		"status":                         u.Status,
		"statusDescription":              u.StatusDescription,
		"bio":                            u.Bio,
		"location":                       u.Location,
		"worldId":                        u.WorldID,
		"travelingToLocation":            u.TravelingToLocation,
		"currentAvatarImageUrl":          u.CurrentAvatarImageURL,
		"currentAvatarThumbnailImageUrl": u.CurrentAvatarThumbnailImageURL,
	}
}

func copySnap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func coalesce(a string, bAny any) string {
	if a != "" {
		return a
	}
	if b, ok := bAny.(string); ok {
		return b
	}
	return ""
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// ensure imports are used
var _ = (*websocket.CloseError)(nil)
var _ = (*http.Client)(nil)
