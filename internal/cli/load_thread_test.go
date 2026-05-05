package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadThreadCommand_Success(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{
					"user": "U001", "text": "first message", "ts": "1700000001.000001",
					"reactions": []map[string]any{{"name": "thumbsup", "count": 3}},
				},
				{
					"user": "U002", "text": "reply here", "ts": "1700000002.000002",
					"files": []map[string]any{{"name": "report.pdf"}},
				},
			},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadThreadCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)

	var result LoadThreadResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	require.Len(t, result.Messages, 2)
	assert.Equal(t, "1700000001.000001", result.Messages[0].Timestamp)
	assert.Equal(t, "U001", result.Messages[0].UserID)
	assert.Equal(t, "first message", result.Messages[0].Text)
	require.Len(t, result.Messages[0].Reactions, 1)
	assert.Equal(t, "thumbsup", result.Messages[0].Reactions[0].Name)
	assert.Equal(t, 3, result.Messages[0].Reactions[0].Count)
	assert.Equal(t, "1700000002.000002", result.Messages[1].Timestamp)
	assert.Equal(t, "U002", result.Messages[1].UserID)
	assert.Equal(t, "reply here", result.Messages[1].Text)
	require.Len(t, result.Messages[1].Files, 1)
	assert.Equal(t, "report.pdf", result.Messages[1].Files[0])
	assert.False(t, result.Truncated)
}

func TestLoadThreadCommand_NoMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{"ok": true, "messages": []any{}},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadThreadCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)

	var result LoadThreadResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Empty(t, result.Messages)
	assert.False(t, result.Truncated)
}

func TestLoadThreadCommand_Unauthorized(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{"ok": false, "error": "invalid_auth"},
		"conversations.history": map[string]any{"ok": false, "error": "invalid_auth"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &LoadThreadCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "C001", "1700000001.000001", time.Time{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestLoadThreadCommand_NoChannelAccess(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{"ok": false, "error": "not_in_channel"},
		"conversations.history": map[string]any{"ok": false, "error": "not_in_channel"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &LoadThreadCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "C001", "1700000001.000001", time.Time{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoChannelAccess)
	assert.Contains(t, err.Error(), "lacks permissions")
}

func TestLoadThreadCommand_WorkspaceNotFound(t *testing.T) {
	cmd := &LoadThreadCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, ClientFactory: DefaultClientFactory()}

	err := cmd.Run(context.Background(), "missing", "C001", "ts", time.Time{})
	require.Error(t, err)
}

func TestLoadThreadCommand_Truncated(t *testing.T) {
	// Simulate 10 pages of results with pagination still available.
	// After maxPages=10 iterations with has_more=true and next_cursor set,
	// the client returns truncated=true.
	responses := map[string]any{
		"conversations.replies": map[string]any{
			"ok":       true,
			"has_more": true,
			"response_metadata": map[string]any{
				"next_cursor": "next_page",
			},
			"messages": []map[string]any{
				{"user": "U001", "text": "msg", "ts": "1700000001.000001"},
			},
		},
	}

	// Create a custom test server that returns the paginated response 10+ times.
	// After 10 requests, it should still have has_more=true and next_cursor set,
	// triggering truncated=true.
	pageCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := r.URL.Path[1:]
		if endpoint == "conversations.replies" {
			pageCount++
			resp := responses["conversations.replies"].(map[string]any)
			w.Header().Set("Content-Type", "application/json")
			// Keep has_more=true and next_cursor set to simulate more pages
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadThreadCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)

	var result LoadThreadResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.True(t, result.Truncated)
}
