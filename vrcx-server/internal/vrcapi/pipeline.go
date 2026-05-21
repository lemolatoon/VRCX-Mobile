package vrcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

const pipelineURL = "wss://pipeline.vrchat.cloud/"

// PipelineEvent is a single decoded message from the VRChat WebSocket pipeline.
type PipelineEvent struct {
	Type    string          // friend-online | friend-offline | friend-active | friend-update | friend-location
	Content json.RawMessage // type-specific JSON
}

// FetchPipelineToken fetches the auth token required to connect to the pipeline.
// Mirrors src/services/websocket.js:53-71 in the original VRCX codebase.
func (c *Client) FetchPipelineToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/auth", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("pipeline token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pipeline token: status %d", resp.StatusCode)
	}

	var body struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("pipeline token: decode: %w", err)
	}
	if !body.OK || body.Token == "" {
		return "", fmt.Errorf("pipeline token: not ok or empty token")
	}
	return body.Token, nil
}

// PipelineConn is an active WebSocket connection to the VRChat pipeline.
type PipelineConn struct {
	ws *websocket.Conn
}

// DialPipeline opens a WebSocket connection to wss://pipeline.vrchat.cloud/?auth=<token>.
// Mirrors src/services/websocket.js:82 in the original VRCX codebase.
func DialPipeline(ctx context.Context, token string) (*PipelineConn, error) {
	url := pipelineURL + "?auth=" + token
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": []string{UserAgent},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dial pipeline: %w", err)
	}
	// VRChat pipeline sends large payloads for friend lists; raise the limit.
	ws.SetReadLimit(1 << 20) // 1 MiB
	return &PipelineConn{ws: ws}, nil
}

// Read blocks until the next pipeline event is available or an error occurs.
// Only friend-* events are returned; other message types are silently skipped.
func (c *PipelineConn) Read(ctx context.Context) (PipelineEvent, error) {
	for {
		_, msg, err := c.ws.Read(ctx)
		if err != nil {
			return PipelineEvent{}, err
		}

		// Outer envelope: {type: string, content: string (JSON-stringified)}
		var outer struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msg, &outer); err != nil {
			continue // malformed message, skip
		}

		switch outer.Type {
		case "friend-online", "friend-offline", "friend-active", "friend-update", "friend-location":
			return PipelineEvent{
				Type:    outer.Type,
				Content: json.RawMessage(outer.Content),
			}, nil
		default:
			// non-friend event (notification, etc.) — skip
		}
	}
}

// Close closes the underlying WebSocket connection.
func (c *PipelineConn) Close() error {
	return c.ws.Close(websocket.StatusNormalClosure, "")
}
