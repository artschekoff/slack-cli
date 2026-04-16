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

func TestListDMs_StartFromFiltersOlderDMs(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"conversations.list": dmListResponse([]map[string]any{
			{"id": "D001", "user": "U001", "is_im": true, "created": 1705363200}, // 2024-01-16
			{"id": "D002", "user": "U002", "is_im": true, "created": 1705276800}, // 2024-01-15
			{"id": "D003", "user": "U003", "is_im": true, "created": 1705190400}, // 2024-01-14
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
	require.Len(t, results, 2, "only DMs created on or after 2024-01-15 should be returned")
	assert.Equal(t, "D001", results[0].ID)
	assert.Equal(t, "D002", results[1].ID)
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
