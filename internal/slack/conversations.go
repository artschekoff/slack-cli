package slack

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// Message represents a single Slack message with resolved metadata.
type Message struct {
	UserID    string
	Text      string
	Timestamp string
	RawTS     string
	Subtype   string // empty for regular messages; e.g. "channel_join", "bot_message"
	BotID     string // non-empty for app/bot messages that omit the "bot_message" subtype
	Reactions []Reaction
	Files     []string
}

// systemMessageSubtypes contains Slack message subtypes that represent
// channel notifications rather than user-authored content.
var systemMessageSubtypes = map[string]struct{}{
	"channel_join":      {},
	"channel_leave":     {},
	"channel_topic":     {},
	"channel_purpose":   {},
	"channel_name":      {},
	"channel_archive":   {},
	"channel_unarchive": {},
	"pinned_item":       {},
	"unpinned_item":     {},
}

// IsSystemMessage reports whether the message is an automated channel
// notification (channel_join, channel_leave, topic change, etc.) rather
// than a user-authored message.
func (m Message) IsSystemMessage() bool {
	_, ok := systemMessageSubtypes[m.Subtype]
	return ok
}

// IsBotMessage reports whether the message was sent by a bot or app integration.
// Slack signals this in two ways: subtype "bot_message", or a non-empty bot_id
// on messages that carry no subtype (common for app integration notifications
// such as Jira "connected this channel" events).
func (m Message) IsBotMessage() bool {
	return m.Subtype == "bot_message" || m.BotID != ""
}

// Reaction holds emoji reaction data.
type Reaction struct {
	Name  string
	Count int
}

type conversationMessage struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Ts        string `json:"ts"`
	Subtype   string `json:"subtype"`
	BotID     string `json:"bot_id"`
	Reactions []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"reactions"`
	Files []struct {
		Name string `json:"name"`
	} `json:"files"`
}

// maxPages caps the number of cursor-paginated requests per call.
// With limit=200 per page this allows up to ~2 000 messages before stopping.
const maxPages = 10

type responseMetadata struct {
	NextCursor string `json:"next_cursor"`
}

// paginatedResponse is the shared response shape for paginated conversations endpoints.
type paginatedResponse struct {
	slackBaseResponse
	Messages         []conversationMessage `json:"messages"`
	HasMore          bool                  `json:"has_more"`
	ResponseMetadata responseMetadata      `json:"response_metadata"`
}

// GetThreadReplies fetches all replies for the thread identified by channelID and threadTS.
// Both the primary (conversations.replies) and fallback (conversations.history) paths
// use cursor-based pagination, capped at maxPages requests each (~2 000 messages).
// Falls back to conversations.history if replies returns an error or <=1 message.
// Permission errors (ErrNoChannelAccess, ErrUnauthorized) short-circuit immediately —
// the fallback would fail with the same error, so there is no point attempting it.
// If the fallback itself fails, any valid messages from the primary call are preserved.
// Returns truncated=true when pagination was capped at maxPages with more data available.
// When startFrom is non-zero, only messages on or after that time are returned.
func (c *Client) GetThreadReplies(ctx context.Context, channelID, threadTS string, startFrom time.Time) ([]Message, bool, error) {
	messages, truncated, err := c.fetchReplies(ctx, channelID, threadTS, startFrom)
	if err != nil {
		if errors.Is(err, ErrNoChannelAccess) || errors.Is(err, ErrUnauthorized) {
			return nil, false, err
		}
		fallback, fbTruncated, fbErr := c.fetchHistory(ctx, channelID, threadTS, startFrom)
		if fbErr != nil {
			return nil, false, fmt.Errorf("conversations.replies: %w; conversations.history: %w", err, fbErr)
		}
		return fallback, fbTruncated, nil
	}
	if len(messages) <= 1 {
		fallback, fbTruncated, fbErr := c.fetchHistory(ctx, channelID, threadTS, startFrom)
		if fbErr != nil {
			return messages, false, nil
		}
		return fallback, fbTruncated, nil
	}
	return messages, truncated, nil
}

// paginatedFetch executes cursor-paginated GET requests against endpoint,
// calling buildParams(cursor) to produce query params for each page.
// Stops after maxPages requests or when HasMore is false.
// Returns truncated=true when stopped at maxPages with more data available.
func (c *Client) paginatedFetch(ctx context.Context, endpoint string, buildParams func(cursor string) url.Values) ([]Message, bool, error) {
	var all []Message
	cursor := ""

	for page := 0; page < maxPages; page++ {
		body, err := c.get(ctx, endpoint, buildParams(cursor))
		if err != nil {
			return nil, false, fmt.Errorf("%s request: %w", endpoint, err)
		}

		var resp paginatedResponse
		if err := unmarshal(body, &resp); err != nil {
			return nil, false, err
		}

		if err := checkResponse(resp.slackBaseResponse); err != nil {
			return nil, false, err
		}

		all = append(all, convertMessages(resp.Messages)...)

		if !resp.HasMore || resp.ResponseMetadata.NextCursor == "" {
			return all, false, nil
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return all, true, nil
}

func (c *Client) fetchReplies(ctx context.Context, channelID, threadTS string, startFrom time.Time) ([]Message, bool, error) {
	return c.paginatedFetch(ctx, "conversations.replies", func(cursor string) url.Values {
		params := url.Values{}
		params.Set("channel", channelID)
		params.Set("ts", threadTS)
		params.Set("limit", "200")
		if !startFrom.IsZero() {
			params.Set("oldest", timeToSlackTS(startFrom))
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		return params
	})
}

func (c *Client) fetchHistory(ctx context.Context, channelID, oldestTS string, startFrom time.Time) ([]Message, bool, error) {
	oldest := oldestTS
	if !startFrom.IsZero() && startFrom.Unix() > tsUnixSec(oldestTS) {
		oldest = timeToSlackTS(startFrom)
	}
	return c.paginatedFetch(ctx, "conversations.history", func(cursor string) url.Values {
		params := url.Values{}
		params.Set("channel", channelID)
		params.Set("oldest", oldest)
		params.Set("limit", "200")
		params.Set("inclusive", "true")
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		return params
	})
}

// timeToSlackTS converts a time.Time to a Slack-format timestamp string.
func timeToSlackTS(t time.Time) string {
	return fmt.Sprintf("%d.000000", t.Unix())
}

func convertMessages(raw []conversationMessage) []Message {
	msgs := make([]Message, 0, len(raw))
	for _, m := range raw {
		reactions := make([]Reaction, 0, len(m.Reactions))
		for _, r := range m.Reactions {
			reactions = append(reactions, Reaction{Name: r.Name, Count: r.Count})
		}

		files := make([]string, 0, len(m.Files))
		for _, f := range m.Files {
			files = append(files, f.Name)
		}

		msgs = append(msgs, Message{
			UserID:    m.User,
			Text:      m.Text,
			Timestamp: FormatTS(m.Ts),
			RawTS:     m.Ts,
			Subtype:   m.Subtype,
			BotID:     m.BotID,
			Reactions: reactions,
			Files:     files,
		})
	}
	return msgs
}
