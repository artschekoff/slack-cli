package slack

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"golang.org/x/sync/singleflight"
)

type userInfoResponse struct {
	slackBaseResponse
	User struct {
		RealName string `json:"real_name"`
		Name     string `json:"name"`
	} `json:"user"`
}

// UserCache resolves Slack user IDs to display names with in-memory caching.
// A singleflight.Group ensures that concurrent requests for the same userID
// collapse into a single API call, preventing duplicate in-flight requests.
type UserCache struct {
	mu     sync.RWMutex
	client *Client
	cache  map[string]string
	group  singleflight.Group
}

// NewUserCache creates a UserCache backed by the given Client.
func NewUserCache(client *Client) *UserCache {
	return &UserCache{
		client: client,
		cache:  make(map[string]string),
	}
}

// Resolve returns the real_name for userID, using the cache when available.
// Concurrent calls for the same userID that miss the cache are collapsed into
// a single API request via singleflight; all callers receive the same result.
func (u *UserCache) Resolve(ctx context.Context, userID string) (string, error) {
	u.mu.RLock()
	if name, ok := u.cache[userID]; ok {
		u.mu.RUnlock()
		return name, nil
	}
	u.mu.RUnlock()

	v, err, _ := u.group.Do(userID, func() (any, error) {
		// Re-check the cache inside the group: a previous flight may have
		// populated it between our read-lock release and group entry.
		u.mu.RLock()
		if name, ok := u.cache[userID]; ok {
			u.mu.RUnlock()
			return name, nil
		}
		u.mu.RUnlock()

		name, err := u.client.GetUserInfo(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("resolving user %s: %w", userID, err)
		}

		u.mu.Lock()
		u.cache[userID] = name
		u.mu.Unlock()

		return name, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// GetUserInfo calls users.info and returns the real_name (falls back to name).
func (c *Client) GetUserInfo(ctx context.Context, userID string) (string, error) {
	params := url.Values{}
	params.Set("user", userID)

	body, err := c.get(ctx, "users.info", params)
	if err != nil {
		return "", fmt.Errorf("users.info request: %w", err)
	}

	var resp userInfoResponse
	if err := unmarshal(body, &resp); err != nil {
		return "", err
	}

	if err := checkResponse(resp.slackBaseResponse); err != nil {
		return "", err
	}

	name := resp.User.RealName
	if name == "" {
		name = resp.User.Name
	}
	if name == "" {
		name = userID
	}
	return name, nil
}
