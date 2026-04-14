package cli

import (
	"bytes"
	"context"
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
	text := out.String()
	assert.Contains(t, text, "U001")
	assert.Contains(t, text, "first message")
	assert.Contains(t, text, ":thumbsup: 3")
	assert.Contains(t, text, "U002")
	assert.Contains(t, text, "report.pdf")
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
	assert.Contains(t, out.String(), "No messages")
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
