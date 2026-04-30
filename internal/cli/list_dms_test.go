package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dmListResponse builds a conversations.list API stub payload for DMs.
func dmListResponse(channels []map[string]any) map[string]any {
	return map[string]any{
		"ok":                true,
		"channels":          channels,
		"response_metadata": map[string]any{"next_cursor": ""},
	}
}

func decodeDMResults(t *testing.T, buf *bytes.Buffer) []dmResult {
	t.Helper()
	var results []dmResult
	require.NoError(t, json.NewDecoder(buf).Decode(&results))
	return results
}

func TestListDMs_Returns1to1DMs(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
			{"id": "D002", "user": "U002", "is_im": true, "created": 1700000000},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 2)
	assert.Equal(t, "D001", results[0].ID)
	assert.True(t, results[0].IsIM)
	assert.Equal(t, "U001", results[0].UserID)
	assert.Equal(t, "D002", results[1].ID)
}

func TestListDMs_ResolveUserNames(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	assert.Equal(t, "Alice Smith", results[0].UserName)
}

func TestListDMs_UserResolutionFallback(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U999", "is_im": true, "created": 1700000000},
		}),
		// users.info intentionally absent — simulates resolution failure
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	assert.Equal(t, "U999", results[0].UserName, "raw user ID must be kept when resolution fails")
}

func TestListDMs_IncludesGroupDMs(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
			{"id": "G001", "name": "mpdm-alice--bob--1", "is_mpim": true, "created": 1700000000},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 2)

	assert.True(t, results[0].IsIM)
	assert.Equal(t, "D001", results[0].ID)

	assert.False(t, results[1].IsIM)
	assert.Equal(t, "G001", results[1].ID)
	assert.Equal(t, "mpdm-alice--bob--1", results[1].Name)
}

func TestListDMs_EmptyResult(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{}),
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	assert.Empty(t, results)
}

func TestListDMs_UnauthorizedError(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": map[string]any{"ok": false, "error": "invalid_auth"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", time.Time{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestListDMs_WorkspaceNotFound(t *testing.T) {
	cmd := &ListDMsCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, ClientFactory: DefaultClientFactory()}

	err := cmd.Run(context.Background(), "missing", time.Time{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials found")
}

// TestListDMs_StartFromIncludesDMWithRecentMessage verifies that a DM with an
// old creation date but a recent message passes the --start-from filter.
func TestListDMs_StartFromIncludesDMWithRecentMessage(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000}, // old creation date
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "recent!", "ts": "1705363200.000001", "client_msg_id": "m1"}, // 2024-01-16 ≥ threshold
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Someone", "name": "someone"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	startFrom := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, cmd.Run(context.Background(), "acme", startFrom))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1, "DM with message on or after startFrom must be included despite old created date")
	assert.Equal(t, "D001", results[0].ID)
	assert.Nil(t, results[0].LastMessage, "--with-messages not set so message data must be absent")
}

// TestListDMs_StartFromExcludesDMWithOldMessage verifies that a DM whose last
// message predates --start-from is excluded from the result.
func TestListDMs_StartFromExcludesDMWithOldMessage(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "old msg", "ts": "1705190400.000001", "client_msg_id": "m1"}, // 2024-01-14 < threshold
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Someone", "name": "someone"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	startFrom := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, cmd.Run(context.Background(), "acme", startFrom))

	results := decodeDMResults(t, &out)
	assert.Empty(t, results, "DM whose last message is before startFrom must be excluded")
}

func TestListDMs_WithMessagesFiltersBotMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"text": "Artem joined Slack — take a second to say hello.", "ts": "1700000002.000001", "bot_id": "B001"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "msg-u001-1"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "hey there!", results[0].LastMessage.Text, "bot message must be skipped by default")
}

func TestListDMs_WithMessagesFiltersSystemMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "U001 joined the channel", "ts": "1700000002.000001", "subtype": "channel_join"},
			{"user": "U001", "text": "real message", "ts": "1700000001.000001", "client_msg_id": "msg-u001-2"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "real message", results[0].LastMessage.Text, "system message must be skipped by default")
}

func TestListDMs_SystemEventsIncludesBotMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"text": "Artem joined Slack — take a second to say hello.", "ts": "1700000002.000001", "bot_id": "B001"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
		SystemEvents: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Contains(t, results[0].LastMessage.Text, "joined Slack", "bot message must be included when --system-events is set")
}

func TestListDMs_AllSystemMessagesExcludesDM(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"text": "Artem joined Slack — take a second to say hello.", "ts": "1700000002.000001", "bot_id": "B001"},
			{"user": "U001", "text": "U001 joined the channel", "ts": "1700000001.000001", "subtype": "channel_join"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	assert.Empty(t, results, "DM with only system/bot messages should be excluded")
}

// TestListDMs_WithMessagesFiltersWorkspaceJoinMessages asserts that the
// "X joined Slack — take a second to say hello." messages are filtered by
// default. In the real Slack API these messages carry no subtype, no bot_id,
// and no client_msg_id (unlike every user-typed message). They appear in every
// DM with the joining user's own ID as the author.
func TestListDMs_WithMessagesFiltersWorkspaceJoinMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			// No subtype, no bot_id, no client_msg_id — Slack workspace-join intro.
			{"user": "U001", "text": "Alice Smith joined Slack — take a second to say hello.", "ts": "1700000002.000001"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "abc-123"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "hey there!", results[0].LastMessage.Text, "workspace-join message with no client_msg_id must be skipped by default")
}

// TestListDMs_WithMessagesFiltersShRoomCreatedMessages asserts that Slack
// "sh_room_created" messages ("X joined Slack — take a second to say hello.")
// are filtered by default. These appear in DMs with the joining user's own ID
// as the author and no bot_id, so they pass the bot/system checks unless the
// subtype is explicitly recognised.
func TestListDMs_WithMessagesFiltersShRoomCreatedMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			// Subtype "sh_room_created": workspace-join notification; user field is
			// the joining user's own ID (no bot_id, no USLACKBOT).
			{"user": "U001", "text": "Alice Smith joined Slack — take a second to say hello.", "ts": "1700000002.000001", "subtype": "sh_room_created"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "msg-u001-3"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "hey there!", results[0].LastMessage.Text, "sh_room_created message must be skipped by default")
}

// TestListDMs_WithMessagesFiltersSlackbotMessages asserts that messages from the
// USLACKBOT user (e.g. "X joined Slack — take a second to say hello.") are
// filtered out by default, even when they carry no subtype and no bot_id.
// These messages are sent by Slackbot directly and would otherwise appear as
// user-authored content.
func TestListDMs_WithMessagesFiltersSlackbotMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			// No subtype, no bot_id — comes from USLACKBOT user directly.
			{"user": "USLACKBOT", "text": "Artem Krivoshchekov joined Slack — take a second to say hello.", "ts": "1700000002.000001"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "msg-u001-4"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "hey there!", results[0].LastMessage.Text, "USLACKBOT message must be skipped by default")
}

// TestListDMs_WithMessagesFiltersJoinedSlackWithClientMsgID reproduces the
// real-world bug where "X joined Slack — take a second to say hello." messages
// carry a client_msg_id (unlike normal system messages), bypassing
// IsAutoGenerated/IsSystemMessage/IsBotMessage/IsSlackbotMessage.
func TestListDMs_WithMessagesFiltersJoinedSlackWithClientMsgID(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			// Real API: has client_msg_id, no subtype, no bot_id, user is the
			// joining user — bypasses all existing filters.
			{"user": "U001", "text": "Alice Smith joined Slack — take a second to say hello.", "ts": "1700000002.000001", "client_msg_id": "deadbeef-1234"},
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "msg-u001-5"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].LastMessage)
	assert.Equal(t, "hey there!", results[0].LastMessage.Text,
		"joined-Slack message with client_msg_id must be filtered by default")
}

func TestListDMs_WithMessages_ExcludesEmptyHistory(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
			{"id": "D002", "user": "U002", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Someone", "name": "someone"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	assert.Empty(t, results, "DMs with empty message history should be excluded when --with-messages is set")
}

func TestListDMs_WithMessages_ExcludesAllSystemOnly(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"text": "Artem joined Slack — take a second to say hello.", "ts": "1700000002.000001", "bot_id": "B001"},
			{"user": "U001", "text": "U001 joined the channel", "ts": "1700000001.000001", "subtype": "channel_join"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	assert.Empty(t, results, "DMs with only system/bot messages should be excluded when --with-messages is set")
}

func TestListDMs_WithMessages_KeepsDMsWithRealMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
			{"id": "D002", "user": "U002", "is_im": true, "created": 1700000000},
		}),
		"conversations.history": channelHistoryResponse([]map[string]any{
			{"user": "U001", "text": "hey there!", "ts": "1700000001.000001", "client_msg_id": "msg-001"},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Someone", "name": "someone"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{
		Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv),
		WithMessages: true,
	}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 2, "DMs with real messages should be kept")
	for _, r := range results {
		require.NotNil(t, r.LastMessage, "each kept DM should have a lastMessage")
	}
}

func TestListDMs_StartFromZeroReturnsAll(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1705363200},
			{"id": "D002", "user": "U002", "is_im": true, "created": 1705190400},
		}),
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Someone", "name": "someone"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &ListDMsCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	require.NoError(t, cmd.Run(context.Background(), "acme", time.Time{}))

	results := decodeDMResults(t, &out)
	require.Len(t, results, 2, "zero startFrom should not filter any results")
}
