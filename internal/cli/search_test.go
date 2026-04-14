package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClientFactory(t *testing.T, srv *httptest.Server) ClientFactory {
	t.Helper()
	return func(creds credentials.Creds) *slack.Client {
		return slack.NewClient(creds.Token, creds.Cookie, slack.WithBaseURL(srv.URL+"/"))
	}
}

func newSlackTestServer(t *testing.T, responses map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := r.URL.Path[1:]
		body, ok := responses[endpoint]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func storeWithCredsForCLI(t *testing.T, workspace, token, cookie string) *credentials.Store {
	t.Helper()
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), workspace, credentials.Creds{Token: token, Cookie: cookie}))
	return store
}

func TestSearchCommand_Success(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"search.messages": map[string]any{
			"ok": true,
			"messages": map[string]any{
				"total": 1,
				"matches": []map[string]any{
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "alice",
						"text":      "hello world",
						"ts":        "1700000000.000001",
						"permalink": "https://acme.slack.com/archives/C001/p1700000000000001",
					},
				},
			},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "hello", 20, time.Time{})
	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "#general")
	assert.Contains(t, text, "alice")
	assert.Contains(t, text, "hello world")
	assert.Contains(t, text, "1 of 1 total")
}

func TestSearchCommand_NoResults(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"search.messages": map[string]any{
			"ok":       true,
			"messages": map[string]any{"total": 0, "matches": []any{}},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &SearchCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "nothing", 20, time.Time{})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No results found")
}

func TestSearchCommand_Unauthorized(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"search.messages": map[string]any{"ok": false, "error": "invalid_auth"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &SearchCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "q", 20, time.Time{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.Contains(t, err.Error(), "expired")
}

func TestSearchCommand_WorkspaceNotFound(t *testing.T) {
	cmd := &SearchCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, ClientFactory: DefaultClientFactory()}

	err := cmd.Run(context.Background(), "missing", "q", 20, time.Time{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials found")
}

func TestSearchCommand_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &SearchCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "q", 20, time.Time{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlackSearch)
}
