package vrcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

// FriendUser is the minimal user payload returned by the friends endpoint.
type FriendUser struct {
	ID                             string `json:"id"`
	DisplayName                    string `json:"displayName"`
	State                          string `json:"state"`
	Status                         string `json:"status"`
	StatusDescription              string `json:"statusDescription"`
	Bio                            string `json:"bio"`
	Location                       string `json:"location"`
	WorldID                        string `json:"worldId"`
	CurrentAvatarImageURL          string `json:"currentAvatarImageUrl"`
	CurrentAvatarThumbnailImageURL string `json:"currentAvatarThumbnailImageUrl"`
}

// FetchAllFriends retrieves all friends (online and offline) by paginating
// GET /api/1/auth/user/friends with n=100.
// This mirrors VRCX's initial friend load done before opening the WS connection.
func (c *Client) FetchAllFriends(ctx context.Context) ([]FriendUser, error) {
	var all []FriendUser
	for _, offline := range []bool{false, true} {
		page, err := c.fetchFriendsPage(ctx, offline)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
	}
	return all, nil
}

func (c *Client) fetchFriendsPage(ctx context.Context, offline bool) ([]FriendUser, error) {
	const pageSize = 100
	var all []FriendUser
	for offset := 0; ; offset += pageSize {
		q := url.Values{}
		q.Set("n", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))
		q.Set("offline", strconv.FormatBool(offline))
		resp, err := c.Do(ctx, "GET", "auth/user/friends?"+q.Encode(), nil, nil)
		if err != nil {
			return nil, fmt.Errorf("friends page: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("friends page: status %d: %s", resp.StatusCode, body)
		}
		var page []FriendUser
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("friends page: decode: %w", err)
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
	}
	return all, nil
}
