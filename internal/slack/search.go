package slack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// SearchMatch represents a single message result from search.messages.
type SearchMatch struct {
	ChannelID   string
	ChannelName string
	Username    string
	Text        string
	Timestamp   string
	Permalink   string
}

// ChannelGroup groups search results belonging to the same channel.
type ChannelGroup struct {
	ChannelID   string
	ChannelName string
	Messages    []SearchMatch
}

// GroupByChannel groups matches by channel ID, preserving first-seen channel
// order and message order within each group.
func GroupByChannel(matches []SearchMatch) []ChannelGroup {
	if len(matches) == 0 {
		return nil
	}

	idx := make(map[string]int, len(matches))
	groups := make([]ChannelGroup, 0, len(matches))

	for _, m := range matches {
		i, exists := idx[m.ChannelID]
		if !exists {
			i = len(groups)
			idx[m.ChannelID] = i
			groups = append(groups, ChannelGroup{
				ChannelID:   m.ChannelID,
				ChannelName: m.ChannelName,
				Messages:    make([]SearchMatch, 0, 4),
			})
		}
		groups[i].Messages = append(groups[i].Messages, m)
	}

	return groups
}

type searchMessagesResponse struct {
	slackBaseResponse
	Messages struct {
		Total   int `json:"total"`
		Matches []struct {
			Channel struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"channel"`
			Username  string `json:"username"`
			Text      string `json:"text"`
			Ts        string `json:"ts"`
			Permalink string `json:"permalink"`
		} `json:"matches"`
	} `json:"messages"`
}

// SearchMessages calls search.messages and returns up to count results sorted by timestamp desc.
// When afterDate is non-zero, results with a timestamp before afterDate are excluded.
func (c *Client) SearchMessages(ctx context.Context, query string, count int, afterDate time.Time) ([]SearchMatch, int, error) {
	if count <= 0 {
		count = 20
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("count", strconv.Itoa(count))
	params.Set("sort", "timestamp")
	params.Set("sort_dir", "desc")

	body, err := c.get(ctx, "search.messages", params)
	if err != nil {
		return nil, 0, fmt.Errorf("search.messages request: %w", err)
	}

	var resp searchMessagesResponse
	if err := unmarshal(body, &resp); err != nil {
		return nil, 0, err
	}

	if err := checkResponse(resp.slackBaseResponse); err != nil {
		return nil, 0, err
	}

	thresholdUnix := afterDate.Unix()
	filtering := !afterDate.IsZero()

	matches := make([]SearchMatch, 0, len(resp.Messages.Matches))
	for _, m := range resp.Messages.Matches {
		if filtering && !tsOnOrAfter(m.Ts, thresholdUnix) {
			continue
		}
		matches = append(matches, SearchMatch{
			ChannelID:   m.Channel.ID,
			ChannelName: m.Channel.Name,
			Username:    m.Username,
			Text:        TruncateText(m.Text),
			Timestamp:   FormatTS(m.Ts),
			Permalink:   m.Permalink,
		})
	}

	return matches, resp.Messages.Total, nil
}

// tsOnOrAfter returns true if the unix seconds portion of a Slack timestamp
// is >= the threshold. Returns true when ts cannot be parsed (include by default).
func tsOnOrAfter(ts string, thresholdUnix int64) bool {
	sec := tsUnixSec(ts)
	if sec == 0 {
		return true
	}
	return sec >= thresholdUnix
}

// TsUnixSec extracts the integer seconds from a Slack timestamp ("1700000000.123456" → 1700000000).
// Returns 0 if the timestamp cannot be parsed.
func TsUnixSec(ts string) int64 { return tsUnixSec(ts) }

// tsUnixSec extracts the integer seconds from a Slack timestamp ("1700000000.123456" → 1700000000).
func tsUnixSec(ts string) int64 {
	dotIdx := indexOf(ts, '.')
	if dotIdx < 0 {
		return 0
	}
	sec, err := strconv.ParseInt(ts[:dotIdx], 10, 64)
	if err != nil {
		return 0
	}
	return sec
}

func indexOf(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}
