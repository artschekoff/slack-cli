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

func newContextTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.replies":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "initial message", "ts": "1700000001.000001"},
					{"user": "U002", "text": "a reply", "ts": "1700000002.000002",
						"reactions": []map[string]any{{"name": "tada", "count": 2}}},
				},
			})
		case "/users.info":
			uid := r.URL.Query().Get("user")
			var name string
			switch uid {
			case "U001":
				name = "Alice"
			case "U002":
				name = "Bob"
			default:
				name = "Unknown"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":   true,
				"user": map[string]any{"real_name": name},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLoadContextCommand_Success(t *testing.T) {
	srv := newContextTestServer(t)
	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadContextCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), LoadContextArgs{
		Workspace:   "acme",
		ChannelID:   "C001",
		ThreadTS:    "1700000001.000001",
		ChannelName: "general",
		SearchQuery: "PROJ-123",
		Permalink:   "https://acme.slack.com/archives/C001/p1700000001000001",
	})
	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "# Slack Context")
	assert.Contains(t, text, "PROJ-123")
	assert.Contains(t, text, "#general")
	assert.Contains(t, text, "Alice")
	assert.Contains(t, text, "Bob")
	assert.Contains(t, text, "initial message")
	assert.Contains(t, text, ":tada: 2")
	assert.Contains(t, text, "https://acme.slack.com/archives/C001/p1700000001000001")
}

func TestLoadContextCommand_NoMessages(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{"ok": true, "messages": []any{}},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadContextCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), LoadContextArgs{
		Workspace: "acme", ChannelID: "C001", ThreadTS: "ts",
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No messages")
}

func TestLoadContextCommand_Unauthorized(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.replies": map[string]any{"ok": false, "error": "invalid_auth"},
		"conversations.history": map[string]any{"ok": false, "error": "invalid_auth"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &LoadContextCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), LoadContextArgs{Workspace: "acme", ChannelID: "C001", ThreadTS: "ts"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestLoadContextCommand_StartFrom_ZeroValue_NoFilter(t *testing.T) {
	srv := newContextTestServer(t)
	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &LoadContextCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), LoadContextArgs{
		Workspace: "acme", ChannelID: "C001", ThreadTS: "ts", StartFrom: time.Time{},
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Alice")
}
