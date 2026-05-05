package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// channelListResponse builds a conversations.list API stub payload.
func channelListResponse(channels []map[string]any) map[string]any {
	return map[string]any{
		"ok":                true,
		"channels":          channels,
		"response_metadata": map[string]any{"next_cursor": ""},
	}
}

// channelHistoryResponse builds a conversations.history API stub payload.
func channelHistoryResponse(messages []map[string]any) map[string]any {
	return map[string]any{
		"ok":                true,
		"messages":          messages,
		"has_more":          false,
		"response_metadata": map[string]any{"next_cursor": ""},
	}
}

func decodeChannelResults(t *testing.T, buf *bytes.Buffer) []channelResult {
	t.Helper()
	var results []channelResult
	require.NoError(t, json.NewDecoder(buf).Decode(&results))
	return results
}

// TestSearchChannels_SubstringMatch verifies that a keyword like "970" matches
// a channel named "epic-970" (i.e. %keyword% style, not exact glob).
func TestSearchChannels_SubstringMatch(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
			{"id": "C002", "name": "general"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "hello world", "ts": "1700000000.000001"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	assert.Equal(t, "C001", results[0].ID)
	assert.Equal(t, "epic-970", results[0].Name)
}

// TestSearchChannels_HyphenNormalization verifies that a pattern with spaces
// matches channel names that use hyphens as separators.
func TestSearchChannels_HyphenNormalization(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	// "epic 970" (space) must match "epic-970" (hyphen).
	require.NoError(t, cmd.Run(context.Background(), "acme", "epic 970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	assert.Equal(t, "C001", results[0].ID)
}

// TestSearchChannels_CaseInsensitive verifies that matching is case-insensitive.
func TestSearchChannels_CaseInsensitive(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "EPIC"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	assert.Equal(t, "C001", results[0].ID)
}

// TestSearchChannels_NoMatch verifies that an empty JSON array is returned when
// no channel name contains the search pattern.
func TestSearchChannels_NoMatch(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
			{"id": "C002", "name": "general"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "random-xyz-nomatch"))

	results := decodeChannelResults(t, &out)
	assert.Empty(t, results)
}

// TestSearchChannels_MessagesIncluded verifies that matched channels carry their
// messages in the output (hierarchical result).
func TestSearchChannels_MessagesIncluded(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "first message", "ts": "1700000001.000001"},
			{"user": "U002", "text": "second message", "ts": "1700000002.000001"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 2)
	assert.Equal(t, "first message", results[0].Messages[0].Text)
	assert.Equal(t, "second message", results[0].Messages[1].Text)
	assert.Equal(t, "U001", results[0].Messages[0].User)
	assert.NotEmpty(t, results[0].Messages[0].Timestamp)
}

// TestSearchChannels_UserResolution verifies that user IDs in messages are
// replaced with display names fetched from users.info.
func TestSearchChannels_UserResolution(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "hello", "ts": "1700000001.000001"},
		}),
		"users.info": map[string]any{
			"ok": true,
			"user": map[string]any{
				"real_name": "Alice Smith",
				"name":      "alice",
			},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1)
	assert.Equal(t, "Alice Smith", results[0].Messages[0].User)
}

// TestSearchChannels_UserResolutionFallback verifies that when users.info fails,
// the raw user ID is kept rather than erroring out.
func TestSearchChannels_UserResolutionFallback(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U999", "text": "message", "ts": "1700000001.000001"},
		}),
		// users.info intentionally absent — simulates resolution failure
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1)
	assert.Equal(t, "U999", results[0].Messages[0].User, "raw user ID must be kept when resolution fails")
}

// TestSearchChannels_SystemMessagesFilteredByDefault verifies that messages with
// a system subtype (channel_join, channel_leave, etc.) are excluded from the
// default (non-verbose) output.
func TestSearchChannels_SystemMessagesFilteredByDefault(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "real message", "ts": "1700000001.000001"},
			{"user": "U002", "text": "U002 joined the channel", "ts": "1700000002.000001", "subtype": "channel_join"},
			{"user": "U003", "text": "U003 has left the channel", "ts": "1700000003.000001", "subtype": "channel_leave"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1, "system messages must be filtered in default mode")
	assert.Equal(t, "real message", results[0].Messages[0].Text)
}

// TestSearchChannels_BotMessagesFilteredByDefault verifies that bot_message
// subtypes (e.g. Jira "connected this channel" notifications) are excluded
// from the default output.
func TestSearchChannels_BotMessagesFilteredByDefault(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "real message", "ts": "1700000001.000001"},
			{"text": "Liora connected this channel to receive updates for JH-970", "ts": "1700000002.000001", "subtype": "bot_message"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1, "bot messages must be filtered in default mode")
	assert.Equal(t, "real message", results[0].Messages[0].Text)
}

// TestSearchChannels_BotIDWithoutSubtypeFiltered verifies that app integration
// messages which carry a bot_id but no "bot_message" subtype (e.g. Jira
// "connected this channel" events) are also filtered by default.
func TestSearchChannels_BotIDWithoutSubtypeFiltered(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "real message", "ts": "1700000001.000001"},
			// bot_id present, no subtype — Jira integration style
			{"bot_id": "B123", "text": "Liora connected this channel to receive updates for JH-970", "ts": "1700000002.000001"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1, "bot_id message without subtype must be filtered by default")
	assert.Equal(t, "real message", results[0].Messages[0].Text)
}

// TestSearchChannels_BotMessagesIncludedWithFlag verifies that bot messages
// appear when BotMessages=true, independently of the SystemEvents flag.
func TestSearchChannels_BotMessagesIncludedWithFlag(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "real message", "ts": "1700000001.000001"},
			{"text": "Liora connected this channel to receive updates for JH-970", "ts": "1700000002.000001", "subtype": "bot_message"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv), BotMessages: true}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Messages, 2, "bot messages must be included when BotMessages=true")
}

// TestSearchChannels_SystemMessagesIncludedWithFlag verifies that system messages
// are present when SystemEvents is true.
func TestSearchChannels_SystemMessagesIncludedWithFlag(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "real message", "ts": "1700000001.000001"},
			{"user": "U002", "text": "U002 joined the channel", "ts": "1700000002.000001", "subtype": "channel_join"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv), SystemEvents: true}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Messages, 2, "system messages must be included in verbose mode")
}

// threadRepliesResponse builds a conversations.replies API stub payload
// containing the parent message followed by reply messages.
func threadRepliesResponse(messages []map[string]any) map[string]any {
	return map[string]any{
		"ok":                true,
		"messages":          messages,
		"has_more":          false,
		"response_metadata": map[string]any{"next_cursor": ""},
	}
}

// TestSearchChannels_ThreadRepliesIncluded verifies that when a top-level message
// has thread replies, those replies are nested under the parent in the output tree.
func TestSearchChannels_ThreadRepliesIncluded(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "jiro-2006"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{
				"user":        "U001",
				"text":        "I guess now we have link throught",
				"ts":          "1700000001.000001",
				"thread_ts":   "1700000001.000001",
				"reply_count": 1,
				"client_msg_id": "msg-001",
			},
		}),
		"conversations.replies": threadRepliesResponse([]map[string]any{
			{
				"user":          "U001",
				"text":          "I guess now we have link throught",
				"ts":            "1700000001.000001",
				"thread_ts":     "1700000001.000001",
				"client_msg_id": "msg-001",
			},
			{
				"user":          "U002",
				"text":          "Im not sure I understand the question.",
				"ts":            "1700000002.000001",
				"thread_ts":     "1700000001.000001",
				"client_msg_id": "msg-002",
			},
		}),
	})

	store := storeWithCredsForCLI(t, "jiro", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "jiro", "jiro 2006"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1, "only the parent message should appear at the top level")
	assert.Equal(t, "I guess now we have link throught", results[0].Messages[0].Text)
	require.Len(t, results[0].Messages[0].Replies, 1, "reply must be nested under the parent message")
	assert.Equal(t, "Im not sure I understand the question.", results[0].Messages[0].Replies[0].Text)
	assert.Equal(t, "U002", results[0].Messages[0].Replies[0].User)
}

// TestSearchChannels_ThreadReplyUserResolution verifies that user IDs in thread
// replies are resolved to display names.
func TestSearchChannels_ThreadReplyUserResolution(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "jiro-2006"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{
				"user":          "U001",
				"text":          "I guess now we have link throught",
				"ts":            "1700000001.000001",
				"thread_ts":     "1700000001.000001",
				"reply_count":   1,
				"client_msg_id": "msg-001",
			},
		}),
		"conversations.replies": threadRepliesResponse([]map[string]any{
			{
				"user":          "U001",
				"text":          "I guess now we have link throught",
				"ts":            "1700000001.000001",
				"thread_ts":     "1700000001.000001",
				"client_msg_id": "msg-001",
			},
			{
				"user":          "U002",
				"text":          "Im not sure I understand the question.",
				"ts":            "1700000002.000001",
				"thread_ts":     "1700000001.000001",
				"client_msg_id": "msg-002",
			},
		}),
		"users.info": map[string]any{
			"ok": true,
			"user": map[string]any{
				"real_name": "Liora Rodill",
				"name":      "liora",
			},
		},
	})

	store := storeWithCredsForCLI(t, "jiro", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "jiro", "jiro 2006"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages[0].Replies, 1)
	assert.Equal(t, "Liora Rodill", results[0].Messages[0].Replies[0].User)
}

// TestSearchChannels_RawTSInOutput verifies that every message in the JSON output
// carries a "rawTs" field populated from the Slack raw timestamp.
func TestSearchChannels_RawTSInOutput(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "hello", "ts": "1746441180.000000"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "970"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 1)
	require.Len(t, results[0].Messages, 1)
	assert.Equal(t, "1746441180.000000", results[0].Messages[0].RawTS, "rawTs must be populated from the Slack timestamp")
}

// TestSearchChannels_MultipleChannelsWithMessages verifies that multiple
// matched channels each carry their own messages.
func TestSearchChannels_MultipleChannelsWithMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": channelListResponse([]map[string]any{
			{"id": "C001", "name": "epic-970"},
			{"id": "C002", "name": "epic-971"},
			{"id": "C003", "name": "general"},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "a message", "ts": "1700000001.000001"},
		}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchChannelsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", "epic"))

	results := decodeChannelResults(t, &out)
	require.Len(t, results, 2)
	assert.Equal(t, "C001", results[0].ID)
	assert.Equal(t, "C002", results[1].ID)
	assert.Len(t, results[0].Messages, 1)
	assert.Len(t, results[1].Messages, 1)
}
