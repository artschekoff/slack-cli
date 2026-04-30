package slack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// Channel is a Slack channel with its ID and name.
type Channel struct {
	ID   string
	Name string
}

// DM represents a Slack direct message conversation (1:1 im or group mpim).
type DM struct {
	ID      string // conversation ID (D… for im, G… for mpim)
	UserID  string // the other user's ID in a 1:1 DM; empty for group DMs
	Name    string // auto-generated name for mpim; empty for im
	IsIM    bool   // true for 1:1 DMs, false for group DMs
	Created int64  // Unix timestamp of conversation creation
}

type conversationsListResponse struct {
	slackBaseResponse
	Channels []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		User    string `json:"user"`
		IsIM    bool   `json:"is_im"`
		Created int64  `json:"created"`
	} `json:"channels"`
	ResponseMetadata responseMetadata `json:"response_metadata"`
}

const maxChannelPages = 20 // up to 20×200 = 4 000 channels

// ListChannels returns all non-archived channels accessible to the token,
// paginating through conversations.list until exhausted or maxChannelPages is reached.
// Slack's conversations.list does not support server-side name filtering, so the full
// channel list (up to maxChannelPages×200 entries) is always fetched; callers must
// filter client-side.
func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var all []Channel
	cursor := ""

	for page := 0; page < maxChannelPages; page++ {
		params := url.Values{}
		params.Set("exclude_archived", "true")
		params.Set("types", "public_channel,private_channel")
		params.Set("limit", "200")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := c.get(ctx, "conversations.list", params)
		if err != nil {
			return nil, fmt.Errorf("conversations.list request: %w", err)
		}

		var resp conversationsListResponse
		if err := unmarshal(body, &resp); err != nil {
			return nil, err
		}

		if err := checkResponse(resp.slackBaseResponse); err != nil {
			return nil, err
		}

		for _, ch := range resp.Channels {
			all = append(all, Channel{ID: ch.ID, Name: ch.Name})
		}

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return all, nil
}

const maxDMPages = 20 // up to 20×200 = 4 000 DM conversations

// ListDMs returns all non-archived direct message conversations (1:1 and group)
// accessible to the token, paginating through conversations.list until exhausted
// or maxDMPages is reached.
func (c *Client) ListDMs(ctx context.Context) ([]DM, error) {
	var all []DM
	cursor := ""

	for page := 0; page < maxDMPages; page++ {
		params := url.Values{}
		params.Set("exclude_archived", "true")
		params.Set("types", "im,mpim")
		params.Set("limit", "200")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := c.get(ctx, "conversations.list", params)
		if err != nil {
			return nil, fmt.Errorf("conversations.list request: %w", err)
		}

		var resp conversationsListResponse
		if err := unmarshal(body, &resp); err != nil {
			return nil, err
		}

		if err := checkResponse(resp.slackBaseResponse); err != nil {
			return nil, err
		}

		for _, ch := range resp.Channels {
			all = append(all, DM{
				ID:      ch.ID,
				UserID:  ch.User,
				Name:    ch.Name,
				IsIM:    ch.IsIM,
				Created: ch.Created,
			})
		}

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return all, nil
}

// GetChannelMessages returns messages from channelID using conversations.history,
// paginated up to 10 pages (see maxPages in conversations.go). Pass limit <= 0 for
// the default page size (200). Returns truncated=true when pagination was capped with
// more data available.
func (c *Client) GetChannelMessages(ctx context.Context, channelID string, limit int) ([]Message, bool, error) {
	if limit <= 0 {
		limit = 200
	}
	return c.paginatedFetch(ctx, "conversations.history", func(cursor string) url.Values {
		params := url.Values{}
		params.Set("channel", channelID)
		params.Set("limit", strconv.Itoa(limit))
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		return params
	})
}
