package slack_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*slack.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := slack.NewClient("xoxc-token", "xoxd-cookie", slack.WithBaseURL(srv.URL+"/"))
	return client, srv
}

// --- FormatTS ---

func TestFormatTS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid timestamp", input: "1700000000.123456", want: "2023-11-14 22:13 UTC"},
		{name: "no dot", input: "1700000000", want: "1700000000"},
		{name: "empty string", input: "", want: ""},
		{name: "non-numeric", input: "abc.def", want: "abc.def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slack.FormatTS(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- TruncateText ---

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "short text unchanged", input: "hello", want: "hello"},
		{name: "empty string", input: "", want: ""},
		{name: "exactly 120 chars", input: strings.Repeat("a", 120), want: strings.Repeat("a", 120)},
		{name: "121 chars truncated", input: strings.Repeat("b", 121), want: strings.Repeat("b", 120) + "…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slack.TruncateText(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- ValidateToken ---

func TestValidateToken_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth.test", r.URL.Path)
		assert.Equal(t, "Bearer xoxc-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"user_id": "U123",
			"user":    "artschekoff",
			"team_id": "T456",
			"team":    "Acme",
		})
	}

	client, _ := newTestServer(t, handler)

	result, err := client.ValidateToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "U123", result.UserID)
	assert.Equal(t, "artschekoff", result.User)
	assert.Equal(t, "T456", result.TeamID)
	assert.Equal(t, "Acme", result.Team)
}

func TestValidateToken_Unauthorized(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "invalid_auth",
		})
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ValidateToken(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, slack.ErrUnauthorized)
}

func TestValidateToken_UnknownError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "some_other_error",
		})
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ValidateToken(context.Background())
	require.ErrorIs(t, err, slack.ErrUnexpected)
	assert.NotContains(t, err.Error(), "some_other_error", "raw Slack error code must not be exposed to callers")
}

// --- SearchMessages ---

func TestSearchMessages_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search.messages", r.URL.Path)
		assert.Equal(t, "my query", r.URL.Query().Get("query"))
		assert.Equal(t, "5", r.URL.Query().Get("count"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": map[string]any{
				"total": 1,
				"matches": []map[string]any{
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "alice",
						"text":      "hello world",
						"ts":        "1700000000.000001",
						"permalink": "https://slack.com/archives/C001/p1700000000000001",
					},
				},
			},
		})
	}

	client, _ := newTestServer(t, handler)

	matches, total, err := client.SearchMessages(context.Background(), "my query", 5, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, matches, 1)
	assert.Equal(t, "C001", matches[0].ChannelID)
	assert.Equal(t, "general", matches[0].ChannelName)
	assert.Equal(t, "alice", matches[0].Username)
	assert.Equal(t, "hello world", matches[0].Text)
}

func TestSearchMessages_EmptyResults(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"messages": map[string]any{"total": 0, "matches": []any{}},
		})
	}

	client, _ := newTestServer(t, handler)

	matches, total, err := client.SearchMessages(context.Background(), "q", 10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, matches)
}

func TestSearchMessages_DefaultCount(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "20", r.URL.Query().Get("count"), "default count should be 20")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"messages": map[string]any{"total": 0, "matches": []any{}},
		})
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.SearchMessages(context.Background(), "q", 0, time.Time{})
	require.NoError(t, err)
}

func TestSearchMessages_UnauthorizedError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "not_authed"})
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.SearchMessages(context.Background(), "q", 10, time.Time{})
	require.ErrorIs(t, err, slack.ErrUnauthorized)
}

// --- GetThreadReplies ---

func TestGetThreadReplies_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "first message", "ts": "1700000001.000001"},
				{"user": "U002", "text": "reply one", "ts": "1700000002.000002"},
			},
		})
	}

	client, _ := newTestServer(t, handler)

	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "U001", msgs[0].UserID)
	assert.Equal(t, "first message", msgs[0].Text)
	assert.Equal(t, "U002", msgs[1].UserID)
}

func TestGetThreadReplies_FallbackToHistory(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			// Return only 1 message — triggers fallback
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U001", "text": "only one", "ts": "1700000001.000001"}},
			})
		} else {
			// conversations.history fallback
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "from history", "ts": "1700000001.000001"},
					{"user": "U002", "text": "history reply", "ts": "1700000002.000002"},
				},
			})
		}
	}

	client, _ := newTestServer(t, handler)

	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "from history", msgs[0].Text)
}

func TestGetThreadReplies_SingleMessage_ReturnedWhenHistoryFails(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "only parent", "ts": "1700000001.000001"},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "ratelimited",
			})
		}
	}

	client, _ := newTestServer(t, handler)

	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err, "valid single message must not be discarded when history fallback fails")
	require.Len(t, msgs, 1)
	assert.Equal(t, "U001", msgs[0].UserID)
	assert.Equal(t, "only parent", msgs[0].Text)
}

func TestGetThreadReplies_NoChannelAccess(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "not_in_channel"})
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.ErrorIs(t, err, slack.ErrNoChannelAccess)
	assert.Equal(t, 1, callCount, "ErrNoChannelAccess must short-circuit: history fallback must not be attempted")
}

func TestGetThreadReplies_UnauthorizedDoesNotFallback(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "token_revoked"})
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.ErrorIs(t, err, slack.ErrUnauthorized)
	assert.Equal(t, 1, callCount, "ErrUnauthorized must short-circuit: history fallback must not be attempted")
}

func TestGetThreadReplies_PaginatesReplies(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "conversations.replies") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "parent", "ts": "1700000001.000001"},
					{"user": "U002", "text": "reply-1", "ts": "1700000002.000002"},
				},
				"has_more": true,
				"response_metadata": map[string]any{
					"next_cursor": "page2cursor",
				},
			})
		} else if cursor == "page2cursor" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U003", "text": "reply-2", "ts": "1700000003.000003"},
					{"user": "U004", "text": "reply-3", "ts": "1700000004.000004"},
				},
				"has_more": false,
			})
		} else {
			t.Errorf("unexpected cursor value: %s", cursor)
		}
	}

	client, _ := newTestServer(t, handler)
	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	require.Len(t, msgs, 4, "all messages from both pages must be returned")
	assert.Equal(t, "parent", msgs[0].Text)
	assert.Equal(t, "reply-1", msgs[1].Text)
	assert.Equal(t, "reply-2", msgs[2].Text)
	assert.Equal(t, "reply-3", msgs[3].Text)
}

func TestGetThreadReplies_PaginatesThreePages(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "conversations.replies") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		cursor := r.URL.Query().Get("cursor")
		switch cursor {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "page-1-msg", "ts": "1700000001.000001"},
				},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "c2"},
			})
		case "c2":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U002", "text": "page-2-msg", "ts": "1700000002.000002"},
				},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "c3"},
			})
		case "c3":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U003", "text": "page-3-msg", "ts": "1700000003.000003"},
				},
				"has_more": false,
			})
		default:
			t.Errorf("unexpected cursor: %s", cursor)
		}
	}

	client, _ := newTestServer(t, handler)
	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	require.Len(t, msgs, 3, "all messages from 3 pages must be returned")
	assert.Equal(t, "page-1-msg", msgs[0].Text)
	assert.Equal(t, "page-2-msg", msgs[1].Text)
	assert.Equal(t, "page-3-msg", msgs[2].Text)
}

func TestGetThreadReplies_HistoryFallbackPaginates(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U001", "text": "only parent", "ts": "1700000001.000001"}},
			})
			return
		}
		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "hist-1", "ts": "1700000001.000001"},
					{"user": "U002", "text": "hist-2", "ts": "1700000002.000002"},
				},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "hcur2"},
			})
		} else if cursor == "hcur2" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U003", "text": "hist-3", "ts": "1700000003.000003"},
				},
				"has_more": false,
			})
		}
	}

	client, _ := newTestServer(t, handler)
	msgs, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	require.Len(t, msgs, 3, "history fallback must paginate and return all messages")
	assert.Equal(t, "hist-1", msgs[0].Text)
	assert.Equal(t, "hist-3", msgs[2].Text)
}

func TestGetThreadReplies_PaginationStopsAtMaxPages(t *testing.T) {
	var pageCount atomic.Int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "conversations.replies") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		page := pageCount.Add(1)
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": fmt.Sprintf("page-%d", page), "ts": fmt.Sprintf("17000000%02d.000001", page)},
			},
			"has_more":          true,
			"response_metadata": map[string]any{"next_cursor": fmt.Sprintf("cursor-%d", page+1)},
		})
	}

	client, _ := newTestServer(t, handler)
	msgs, truncated, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)

	assert.LessOrEqual(t, int(pageCount.Load()), 10,
		"pagination must stop after a bounded number of pages to prevent infinite loops")
	assert.Greater(t, len(msgs), 1, "should have fetched multiple pages of messages")
	assert.True(t, truncated, "must report truncation when replies are capped at maxPages")
}

func TestGetThreadReplies_NotTruncated_WhenAllFetched(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "first message", "ts": "1700000001.000001"},
				{"user": "U002", "text": "reply one", "ts": "1700000002.000002"},
			},
		})
	}

	client, _ := newTestServer(t, handler)
	_, truncated, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	assert.False(t, truncated, "must not report truncation when all messages were fetched")
}

// --- GetUserInfo ---

func TestGetUserInfo_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "U123", r.URL.Query().Get("user"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		})
	}

	client, _ := newTestServer(t, handler)

	name, err := client.GetUserInfo(context.Background(), "U123")
	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", name)
}

func TestGetUserInfo_FallbackToName(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "", "name": "alice"},
		})
	}

	client, _ := newTestServer(t, handler)

	name, err := client.GetUserInfo(context.Background(), "U999")
	require.NoError(t, err)
	assert.Equal(t, "alice", name)
}

func TestGetUserInfo_FallbackToUserID(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "", "name": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	name, err := client.GetUserInfo(context.Background(), "U999")
	require.NoError(t, err)
	assert.Equal(t, "U999", name)
}

// --- UserCache ---

func TestUserCache_CachesResults(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Bob Jones"},
		})
	}

	client, _ := newTestServer(t, handler)
	cache := slack.NewUserCache(client)

	name1, err := cache.Resolve(context.Background(), "U111")
	require.NoError(t, err)
	assert.Equal(t, "Bob Jones", name1)

	name2, err := cache.Resolve(context.Background(), "U111")
	require.NoError(t, err)
	assert.Equal(t, "Bob Jones", name2)

	assert.Equal(t, 1, callCount, "should only call Slack API once")
}

func TestUserCache_DifferentUsers(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		name := "Unknown"
		if r.URL.Query().Get("user") == "U111" {
			name = "Alice"
		} else if r.URL.Query().Get("user") == "U222" {
			name = "Bob"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": name},
		})
	}

	client, _ := newTestServer(t, handler)
	cache := slack.NewUserCache(client)

	alice, err := cache.Resolve(context.Background(), "U111")
	require.NoError(t, err)
	assert.Equal(t, "Alice", alice)

	bob, err := cache.Resolve(context.Background(), "U222")
	require.NoError(t, err)
	assert.Equal(t, "Bob", bob)
}

func TestUserCache_ConcurrentResolveSameUser_SingleAPICall(t *testing.T) {
	const goroutines = 10
	var apiCallCount atomic.Int32

	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "users.info") {
			return
		}
		apiCallCount.Add(1)
		// Delay so goroutines overlap and all reach the cache-miss branch
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith"},
		})
	}

	client, _ := newTestServer(t, handler)
	cache := slack.NewUserCache(client)

	start := make(chan struct{})
	results := make([]string, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = cache.Resolve(context.Background(), "U111")
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d failed", i)
		assert.Equal(t, "Alice Smith", results[i])
	}
	assert.Equal(t, int32(1), apiCallCount.Load(),
		"singleflight must deduplicate concurrent requests for the same userID")
}

func TestUserCache_Resolve_ReturnsEmptyOnError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	client, _ := newTestServer(t, handler)
	cache := slack.NewUserCache(client)

	name, err := cache.Resolve(context.Background(), "U999")
	require.Error(t, err)
	assert.Empty(t, name,
		"Resolve must return an empty string on error, not the raw userID; callers own the fallback")
}

func TestUserCache_Resolve_DoesNotCacheFailures(t *testing.T) {
	attempt := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Recovered Alice"},
		})
	}

	client, _ := newTestServer(t, handler)
	cache := slack.NewUserCache(client)

	name, err := cache.Resolve(context.Background(), "U111")
	require.Error(t, err)
	assert.Empty(t, name)

	name, err = cache.Resolve(context.Background(), "U111")
	require.NoError(t, err)
	assert.Equal(t, "Recovered Alice", name, "retry after transient failure should succeed")
	assert.Equal(t, 2, attempt)
}

// --- parseRetryAfter ---

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "empty header defaults to 1", header: "", want: 1},
		{name: "zero value defaults to 1", header: "0", want: 1},
		{name: "negative value defaults to 1", header: "-5", want: 1},
		{name: "non-numeric defaults to 1", header: "abc", want: 1},
		{name: "whitespace-padded parses correctly", header: "  30  ", want: 30},
		{name: "normal value within cap", header: "10", want: 10},
		{name: "value at cap boundary", header: "60", want: 60},
		{name: "value above cap is clamped", header: "61", want: 60},
		{name: "large value is clamped to max", header: "99999", want: 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slack.ParseRetryAfter(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- RateLimiting ---

func TestClient_RateLimit_RetriesOnce(t *testing.T) {
	attempt := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"user_id": "U1",
			"user":    "testuser",
			"team_id": "T1",
			"team":    "Test",
		})
	}

	client, _ := newTestServer(t, handler)

	result, err := client.ValidateToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "testuser", result.User)
	assert.Equal(t, 2, attempt, "should have retried once after 429")
}

func TestClient_RateLimit_FailsAfterRetry(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ValidateToken(context.Background())
	require.ErrorIs(t, err, slack.ErrRateLimited)
}

// --- paginatedFetch helper ---

func TestPaginatedFetch_SinglePage(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "msg-1", "ts": "1700000001.000001"},
				{"user": "U002", "text": "msg-2", "ts": "1700000002.000002"},
			},
			"has_more": false,
		})
	}
	client, _ := newTestServer(t, handler)

	msgs, _, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(cursor string) url.Values {
		p := url.Values{}
		p.Set("channel", "C001")
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		return p
	})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "msg-1", msgs[0].Text)
	assert.Equal(t, "msg-2", msgs[1].Text)
}

func TestPaginatedFetch_MultiPage(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":                true,
				"messages":          []map[string]any{{"user": "U001", "text": "page-1", "ts": "1700000001.000001"}},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "c2"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U002", "text": "page-2", "ts": "1700000002.000002"}},
				"has_more": false,
			})
		}
	}
	client, _ := newTestServer(t, handler)

	msgs, _, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(cursor string) url.Values {
		p := url.Values{}
		p.Set("channel", "C001")
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		return p
	})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "page-1", msgs[0].Text)
	assert.Equal(t, "page-2", msgs[1].Text)
}

func TestPaginatedFetch_StopsAtMaxPages(t *testing.T) {
	var pageCount atomic.Int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		page := pageCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"messages":          []map[string]any{{"user": "U001", "text": fmt.Sprintf("p%d", page), "ts": fmt.Sprintf("17000000%02d.000001", page)}},
			"has_more":          true,
			"response_metadata": map[string]any{"next_cursor": fmt.Sprintf("c%d", page+1)},
		})
	}
	client, _ := newTestServer(t, handler)

	msgs, truncated, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(cursor string) url.Values {
		p := url.Values{}
		p.Set("channel", "C001")
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		return p
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, int(pageCount.Load()), slack.MaxPages, "must stop after maxPages requests")
	assert.Len(t, msgs, slack.MaxPages, "must have exactly maxPages messages")
	assert.True(t, truncated, "must report truncation when stopped at maxPages with more data available")
}

func TestPaginatedFetch_ReturnsTruncatedFalse_WhenComplete(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"messages": []map[string]any{{"user": "U001", "text": "only-page", "ts": "1700000001.000001"}},
			"has_more": false,
		})
	}
	client, _ := newTestServer(t, handler)

	_, truncated, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(cursor string) url.Values {
		return url.Values{"channel": []string{"C001"}}
	})
	require.NoError(t, err)
	assert.False(t, truncated, "must not report truncation when all pages were fetched")
}

func TestPaginatedFetch_PropagatesHTTPError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	client, _ := newTestServer(t, handler)

	_, _, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(_ string) url.Values {
		return url.Values{"channel": []string{"C001"}}
	})
	require.Error(t, err)
}

func TestPaginatedFetch_PropagatesSlackError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "not_in_channel"})
	}
	client, _ := newTestServer(t, handler)

	_, _, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(_ string) url.Values {
		return url.Values{"channel": []string{"C001"}}
	})
	require.ErrorIs(t, err, slack.ErrNoChannelAccess)
}

func TestPaginatedFetch_PassesCursorOnSubsequentPages(t *testing.T) {
	var mu sync.Mutex
	var receivedCursors []string

	handler := func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		mu.Lock()
		receivedCursors = append(receivedCursors, cursor)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if cursor == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":                true,
				"messages":          []map[string]any{{"user": "U001", "text": "p1", "ts": "1700000001.000001"}},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "expected-cursor"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U002", "text": "p2", "ts": "1700000002.000002"}},
				"has_more": false,
			})
		}
	}
	client, _ := newTestServer(t, handler)

	_, _, err := client.ExportedPaginatedFetch(context.Background(), "conversations.test", func(cursor string) url.Values {
		p := url.Values{}
		p.Set("channel", "C001")
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		return p
	})
	require.NoError(t, err)
	require.Len(t, receivedCursors, 2)
	assert.Equal(t, "", receivedCursors[0], "first request must not send a cursor")
	assert.Equal(t, "expected-cursor", receivedCursors[1], "second request must forward cursor from previous response")
}

// --- fetchReplies / fetchHistory parameter contracts ---

func TestFetchReplies_SendsCorrectParams(t *testing.T) {
	var capturedParams url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "conversations.replies") {
			capturedParams = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "parent", "ts": "1700000001.000001"},
				{"user": "U002", "text": "reply", "ts": "1700000002.000002"},
			},
			"has_more": false,
		})
	}
	client, _ := newTestServer(t, handler)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "C001", capturedParams.Get("channel"))
	assert.Equal(t, "1700000001.000001", capturedParams.Get("ts"))
	assert.Equal(t, "200", capturedParams.Get("limit"))
	assert.Empty(t, capturedParams.Get("oldest"), "fetchReplies must not set oldest param")
	assert.Empty(t, capturedParams.Get("inclusive"), "fetchReplies must not set inclusive param")
}

func TestFetchHistory_SendsCorrectParams(t *testing.T) {
	var capturedParams url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			// return single message to trigger history fallback
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U001", "text": "only", "ts": "1700000001.000001"}},
				"has_more": false,
			})
			return
		}
		capturedParams = r.URL.Query()
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "hist-1", "ts": "1700000001.000001"},
				{"user": "U002", "text": "hist-2", "ts": "1700000002.000002"},
			},
			"has_more": false,
		})
	}
	client, _ := newTestServer(t, handler)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "C001", capturedParams.Get("channel"))
	assert.Equal(t, "1700000001.000001", capturedParams.Get("oldest"))
	assert.Equal(t, "true", capturedParams.Get("inclusive"))
	assert.Equal(t, "200", capturedParams.Get("limit"))
	assert.Empty(t, capturedParams.Get("ts"), "fetchHistory must not set ts param")
}

// --- checkResponse error-code mapping ---

func TestCheckResponse_UnknownCodeNotExposed(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr error
	}{
		{name: "ratelimited maps to ErrRateLimited", code: "ratelimited", wantErr: slack.ErrRateLimited},
		{name: "completely unknown code maps to ErrUnexpected", code: "some_unknown_code_xyz", wantErr: slack.ErrUnexpected},
		{name: "another arbitrary code maps to ErrUnexpected", code: "ekm_access_denied", wantErr: slack.ErrUnexpected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := slack.SlackBaseResponse{OK: false, Error: tt.code}
			err := slack.CheckResponse(base)
			require.Error(t, err)
			require.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), tt.code, "raw Slack error code must not appear in the error message")
		})
	}
}

func TestCheckResponse_KnownCodesMapToSentinels(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr error
	}{
		{name: "not_in_channel", code: "not_in_channel", wantErr: slack.ErrNoChannelAccess},
		{name: "channel_not_found", code: "channel_not_found", wantErr: slack.ErrNoChannelAccess},
		{name: "missing_scope", code: "missing_scope", wantErr: slack.ErrNoChannelAccess},
		{name: "invalid_auth", code: "invalid_auth", wantErr: slack.ErrUnauthorized},
		{name: "not_authed", code: "not_authed", wantErr: slack.ErrUnauthorized},
		{name: "account_inactive", code: "account_inactive", wantErr: slack.ErrUnauthorized},
		{name: "token_revoked", code: "token_revoked", wantErr: slack.ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := slack.SlackBaseResponse{OK: false, Error: tt.code}
			err := slack.CheckResponse(base)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestCheckResponse_OKResponseReturnsNil(t *testing.T) {
	base := slack.SlackBaseResponse{OK: true}
	require.NoError(t, slack.CheckResponse(base))
}

// --- HTTP status code validation ---

func TestClient_NonOKHTTPStatus_ReturnsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "internal server error", statusCode: http.StatusInternalServerError},
		{name: "forbidden", statusCode: http.StatusForbidden},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable},
		{name: "not found", statusCode: http.StatusNotFound},
		{name: "bad gateway", statusCode: http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}

			client, _ := newTestServer(t, handler)

			_, err := client.ValidateToken(context.Background())
			require.Error(t, err)
			assert.ErrorContains(t, err, fmt.Sprintf("HTTP %d", tt.statusCode))
		})
	}
}

func TestClient_NonOKHTTPStatus_DoesNotReadBodyAsJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// A real server error page — not valid Slack JSON
		fmt.Fprint(w, "<html>Internal Server Error</html>")
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.SearchMessages(context.Background(), "query", 5, time.Time{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "HTTP 500",
		"should return HTTP status error, not a JSON decode error")
}

func TestClient_NonOKHTTPStatus_RateLimitStillRetries(t *testing.T) {
	attempt := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Second attempt also fails with 500 — should propagate HTTP error
		w.WriteHeader(http.StatusInternalServerError)
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ValidateToken(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "HTTP 500")
	assert.Equal(t, 2, attempt, "rate-limit retry must still happen before the 500 is surfaced")
}

// --- SearchMessages afterDate filtering ---

func TestSearchMessages_WithAfterDate_FiltersOlderResults(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": map[string]any{
				"total": 3,
				"matches": []map[string]any{
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "alice",
						"text":      "new message",
						"ts":        "1705363200.000001", // 2024-01-16 00:00 UTC
						"permalink": "https://acme.slack.com/archives/C001/p1705363200000001",
					},
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "bob",
						"text":      "on the date",
						"ts":        "1705276800.000002", // 2024-01-15 00:00 UTC
						"permalink": "https://acme.slack.com/archives/C001/p1705276800000002",
					},
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "charlie",
						"text":      "old message",
						"ts":        "1705190400.000003", // 2024-01-14 00:00 UTC
						"permalink": "https://acme.slack.com/archives/C001/p1705190400000003",
					},
				},
			},
		})
	}

	client, _ := newTestServer(t, handler)
	afterDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	matches, total, err := client.SearchMessages(context.Background(), "test", 10, afterDate)
	require.NoError(t, err)
	assert.Equal(t, 3, total, "total from API should be unfiltered")
	require.Len(t, matches, 2, "only messages on or after 2024-01-15 should be returned")
	assert.Equal(t, "new message", matches[0].Text)
	assert.Equal(t, "on the date", matches[1].Text)
}

func TestSearchMessages_WithZeroAfterDate_ReturnsAll(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": map[string]any{
				"total": 2,
				"matches": []map[string]any{
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "alice",
						"text":      "msg-a",
						"ts":        "1705363200.000001",
						"permalink": "https://acme.slack.com/archives/C001/p1705363200000001",
					},
					{
						"channel":   map[string]any{"id": "C001", "name": "general"},
						"username":  "bob",
						"text":      "msg-b",
						"ts":        "1705190400.000002",
						"permalink": "https://acme.slack.com/archives/C001/p1705190400000002",
					},
				},
			},
		})
	}

	client, _ := newTestServer(t, handler)
	matches, _, err := client.SearchMessages(context.Background(), "test", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, matches, 2, "zero afterDate should not filter any results")
}

// --- GetThreadReplies startFrom filtering ---

func TestGetThreadReplies_WithStartFrom_SetsOldestOnReplies(t *testing.T) {
	var capturedParams url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "conversations.replies") {
			capturedParams = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "parent", "ts": "1700000001.000001"},
				{"user": "U002", "text": "reply", "ts": "1700000002.000002"},
			},
			"has_more": false,
		})
	}

	client, _ := newTestServer(t, handler)
	startFrom := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", startFrom)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d.000000", startFrom.Unix()), capturedParams.Get("oldest"),
		"conversations.replies should receive oldest param from startFrom")
}

func TestGetThreadReplies_WithZeroStartFrom_NoOldestOnReplies(t *testing.T) {
	var capturedParams url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "conversations.replies") {
			capturedParams = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"user": "U001", "text": "parent", "ts": "1700000001.000001"},
				{"user": "U002", "text": "reply", "ts": "1700000002.000002"},
			},
			"has_more": false,
		})
	}

	client, _ := newTestServer(t, handler)

	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, capturedParams.Get("oldest"),
		"zero startFrom should not set oldest param on conversations.replies")
}

func TestGetThreadReplies_WithStartFrom_HistoryUsesLaterTimestamp(t *testing.T) {
	var capturedHistoryParams url.Values
	startFrom := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC) // Way after thread TS (2023-11-14)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U001", "text": "only", "ts": "1700000001.000001"}},
			})
		} else {
			capturedHistoryParams = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "hist", "ts": "1717200000.000001"},
					{"user": "U002", "text": "hist-2", "ts": "1717200001.000001"},
				},
			})
		}
	}

	client, _ := newTestServer(t, handler)
	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", startFrom)
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%d.000000", startFrom.Unix()), capturedHistoryParams.Get("oldest"),
		"when startFrom > threadTS, history should use startFrom as oldest")
}

func TestGetThreadReplies_WithStartFrom_HistoryKeepsThreadTS_WhenLater(t *testing.T) {
	var capturedHistoryParams url.Values
	startFrom := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC) // Before thread TS (2023-11-14)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "conversations.replies") {
			json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"messages": []map[string]any{{"user": "U001", "text": "only", "ts": "1700000001.000001"}},
			})
		} else {
			capturedHistoryParams = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"user": "U001", "text": "hist", "ts": "1700000001.000001"},
					{"user": "U002", "text": "hist-2", "ts": "1700000002.000001"},
				},
			})
		}
	}

	client, _ := newTestServer(t, handler)
	_, _, err := client.GetThreadReplies(context.Background(), "C001", "1700000001.000001", startFrom)
	require.NoError(t, err)

	assert.Equal(t, "1700000001.000001", capturedHistoryParams.Get("oldest"),
		"when startFrom < threadTS, history should keep threadTS as oldest")
}

// --- IsWorkspaceJoinIntro ---

func TestMessage_IsWorkspaceJoinIntro(t *testing.T) {
	tests := []struct {
		name string
		msg  slack.Message
		want bool
	}{
		{
			name: "standard workspace-join intro",
			msg:  slack.Message{UserID: "U001", Text: "Alice Smith joined Slack — take a second to say hello."},
			want: true,
		},
		{
			name: "workspace-join intro with client_msg_id",
			msg:  slack.Message{UserID: "U001", Text: "Alice Smith joined Slack — take a second to say hello.", ClientMsgID: "deadbeef"},
			want: true,
		},
		{
			name: "regular user message",
			msg:  slack.Message{UserID: "U001", Text: "hey there!", ClientMsgID: "abc-123"},
			want: false,
		},
		{
			name: "partial match should not trigger",
			msg:  slack.Message{UserID: "U001", Text: "I just joined Slack"},
			want: false,
		},
		{
			name: "empty text",
			msg:  slack.Message{UserID: "U001", Text: ""},
			want: false,
		},
		{
			name: "message mentioning joined Slack in conversation",
			msg:  slack.Message{UserID: "U001", Text: "When I joined Slack it was great", ClientMsgID: "abc"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.IsWorkspaceJoinIntro()
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- tsOnOrAfter helper ---

func TestTsOnOrAfter(t *testing.T) {
	threshold := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).Unix() // 1705276800

	tests := []struct {
		name string
		ts   string
		want bool
	}{
		{name: "after threshold", ts: "1705363200.000001", want: true},
		{name: "exactly at threshold", ts: "1705276800.000000", want: true},
		{name: "before threshold", ts: "1705190400.000003", want: false},
		{name: "unparseable returns true", ts: "not-a-ts", want: true},
		{name: "no dot returns true", ts: "1700000000", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slack.TsOnOrAfter(tt.ts, threshold)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- timeToSlackTS helper ---

func TestTimeToSlackTS(t *testing.T) {
	ts := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got := slack.TimeToSlackTS(ts)
	assert.Equal(t, "1705276800.000000", got)
}

// --- GroupByChannel ---

func TestGroupByChannel_EmptyInput(t *testing.T) {
	groups := slack.GroupByChannel(nil)
	assert.Empty(t, groups)
}

func TestGroupByChannel_SingleChannel(t *testing.T) {
	matches := []slack.SearchMatch{
		{ChannelID: "C001", ChannelName: "general", Username: "alice", Text: "msg-1"},
		{ChannelID: "C001", ChannelName: "general", Username: "bob", Text: "msg-2"},
	}

	groups := slack.GroupByChannel(matches)
	require.Len(t, groups, 1)
	assert.Equal(t, "C001", groups[0].ChannelID)
	assert.Equal(t, "general", groups[0].ChannelName)
	require.Len(t, groups[0].Messages, 2)
	assert.Equal(t, "msg-1", groups[0].Messages[0].Text)
	assert.Equal(t, "msg-2", groups[0].Messages[1].Text)
}

func TestGroupByChannel_MultipleChannels(t *testing.T) {
	matches := []slack.SearchMatch{
		{ChannelID: "C001", ChannelName: "general", Username: "alice", Text: "msg-1"},
		{ChannelID: "C002", ChannelName: "dev", Username: "bob", Text: "msg-2"},
		{ChannelID: "C001", ChannelName: "general", Username: "charlie", Text: "msg-3"},
		{ChannelID: "C003", ChannelName: "random", Username: "dave", Text: "msg-4"},
		{ChannelID: "C002", ChannelName: "dev", Username: "eve", Text: "msg-5"},
	}

	groups := slack.GroupByChannel(matches)
	require.Len(t, groups, 3)

	assert.Equal(t, "C001", groups[0].ChannelID)
	assert.Equal(t, "general", groups[0].ChannelName)
	require.Len(t, groups[0].Messages, 2)
	assert.Equal(t, "msg-1", groups[0].Messages[0].Text)
	assert.Equal(t, "msg-3", groups[0].Messages[1].Text)

	assert.Equal(t, "C002", groups[1].ChannelID)
	assert.Equal(t, "dev", groups[1].ChannelName)
	require.Len(t, groups[1].Messages, 2)
	assert.Equal(t, "msg-2", groups[1].Messages[0].Text)
	assert.Equal(t, "msg-5", groups[1].Messages[1].Text)

	assert.Equal(t, "C003", groups[2].ChannelID)
	assert.Equal(t, "random", groups[2].ChannelName)
	require.Len(t, groups[2].Messages, 1)
	assert.Equal(t, "msg-4", groups[2].Messages[0].Text)
}

func TestGroupByChannel_PreservesFirstSeenOrder(t *testing.T) {
	matches := []slack.SearchMatch{
		{ChannelID: "C003", ChannelName: "random"},
		{ChannelID: "C001", ChannelName: "general"},
		{ChannelID: "C002", ChannelName: "dev"},
		{ChannelID: "C001", ChannelName: "general"},
	}

	groups := slack.GroupByChannel(matches)
	require.Len(t, groups, 3)
	assert.Equal(t, "C003", groups[0].ChannelID, "first channel seen should come first")
	assert.Equal(t, "C001", groups[1].ChannelID)
	assert.Equal(t, "C002", groups[2].ChannelID)
}

// --- ListDMs ---

func TestListDMs_Returns1to1AndGroupDMs(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conversations.list", r.URL.Path)
		assert.Equal(t, "im,mpim", r.URL.Query().Get("types"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
				{"id": "G001", "name": "mpdm-alice--bob--1", "is_mpim": true, "created": 1700100000},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	dms, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Len(t, dms, 2)

	assert.Equal(t, "D001", dms[0].ID)
	assert.Equal(t, "U001", dms[0].UserID)
	assert.True(t, dms[0].IsIM)
	assert.Equal(t, int64(1700000000), dms[0].Created)

	assert.Equal(t, "G001", dms[1].ID)
	assert.Equal(t, "mpdm-alice--bob--1", dms[1].Name)
	assert.False(t, dms[1].IsIM)
	assert.Equal(t, int64(1700100000), dms[1].Created)
}

func TestListDMs_PaginatesThroughPages(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "D001", "user": "U001", "is_im": true, "created": 1700000000},
				},
				"response_metadata": map[string]any{"next_cursor": "page2"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "D002", "user": "U002", "is_im": true, "created": 1700000000},
				},
				"response_metadata": map[string]any{"next_cursor": ""},
			})
		}
	}

	client, _ := newTestServer(t, handler)

	dms, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Len(t, dms, 2)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, "D001", dms[0].ID)
	assert.Equal(t, "D002", dms[1].ID)
}

func TestListDMs_EmptyResult(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"channels":          []any{},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	dms, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Empty(t, dms)
}

func TestListDMs_UnauthorizedError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid_auth"})
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ListDMs(context.Background(), time.Time{})
	require.ErrorIs(t, err, slack.ErrUnauthorized)
}

func TestListDMs_SendsCorrectTypes(t *testing.T) {
	var capturedTypes string
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedTypes = r.URL.Query().Get("types")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"channels":          []any{},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "im,mpim", capturedTypes,
		"ListDMs must request im and mpim conversation types")
}

func TestListDMs_ExcludesArchived(t *testing.T) {
	var capturedExcludeArchived string
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedExcludeArchived = r.URL.Query().Get("exclude_archived")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"channels":          []any{},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	_, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "true", capturedExcludeArchived)
}

func TestListDMs_StartDateFiltersOlderDMs(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "D001", "user": "U001", "is_im": true, "created": 1705363200}, // 2024-01-16
				{"id": "D002", "user": "U002", "is_im": true, "created": 1705276800}, // 2024-01-15
				{"id": "D003", "user": "U003", "is_im": true, "created": 1705190400}, // 2024-01-14
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	dms, err := client.ListDMs(context.Background(), startDate)
	require.NoError(t, err)
	require.Len(t, dms, 2, "only DMs created on or after 2024-01-15 should be returned")
	assert.Equal(t, "D001", dms[0].ID)
	assert.Equal(t, "D002", dms[1].ID)
}

func TestListDMs_StartDateZeroReturnsAll(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "D001", "user": "U001", "is_im": true, "created": 1705363200},
				{"id": "D002", "user": "U002", "is_im": true, "created": 1705190400},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)

	dms, err := client.ListDMs(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Len(t, dms, 2, "zero startDate should not filter any results")
}

func TestListDMs_StartDateExactBoundary(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "D001", "user": "U001", "is_im": true, "created": 1705276800}, // exactly 2024-01-15 00:00 UTC
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	dms, err := client.ListDMs(context.Background(), startDate)
	require.NoError(t, err)
	require.Len(t, dms, 1, "DM created exactly at startDate boundary must be included")
}

func TestListDMs_StartDateFiltersMPIMToo(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "D001", "user": "U001", "is_im": true, "created": 1705363200},           // 2024-01-16 — keep
				{"id": "G001", "name": "mpdm-a--b--1", "is_mpim": true, "created": 1705190400}, // 2024-01-14 — filter out
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	}

	client, _ := newTestServer(t, handler)
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	dms, err := client.ListDMs(context.Background(), startDate)
	require.NoError(t, err)
	require.Len(t, dms, 1, "mpim DMs older than startDate must also be filtered out")
	assert.Equal(t, "D001", dms[0].ID)
}
