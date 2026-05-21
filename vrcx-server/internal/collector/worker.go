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

// pipelineUserPayload is the shape of content.user in most pipeline events.
type pipelineUserPayload struct {
	ID                              string `json:"id"`
	DisplayName                     string `json:"displayName"`
	State                           string `json:"state"`
	Status                          string `json:"status"`
	StatusDescription               string `json:"statusDescription"`
	Bio                             string `json:"bio"`
	Location                        string `json:"location"`
	WorldID                         string `json:"worldId"`
	TravelingToLocation             string `json:"travelingToLocation"`
	CurrentAvatarImageURL           string `json:"currentAvatarImageUrl"`
	CurrentAvatarThumbnailImageURL  string `json:"currentAvatarThumbnailImageUrl"`
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
			// No cache — just record offline state with minimal info.
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
		// state from the user payload itself, or preserve cached
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
		// First event for this user — just seed the cache; no feed entries.
		return w.feedStore.SaveCache(ctx, w.vrchatUserID, vrcID, nextSnap, nextState)
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
		"id":                              u.ID,
		"displayName":                     u.DisplayName,
		"state":                           u.State,
		"status":                          u.Status,
		"statusDescription":               u.StatusDescription,
		"bio":                             u.Bio,
		"location":                        u.Location,
		"worldId":                         u.WorldID,
		"travelingToLocation":             u.TravelingToLocation,
		"currentAvatarImageUrl":           u.CurrentAvatarImageURL,
		"currentAvatarThumbnailImageUrl":  u.CurrentAvatarThumbnailImageURL,
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

// ensure websocket.CloseError is handled cleanly in wsjobs
var _ = (*websocket.CloseError)(nil)
var _ = (*http.Client)(nil)
