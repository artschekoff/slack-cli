package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"golang.org/x/sync/errgroup"
)

const (
	maxConcurrentUserResolves   = 5  // goroutine limit for concurrent user ID resolution
	maxConcurrentChannelFetches = 10 // goroutine limit for concurrent channel message fetching
)

// messageResult is one message inside a channelResult.
type messageResult struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// channelResult is one channel in the JSON output of search-channels,
// including its messages for hierarchical output.
type channelResult struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Messages  []messageResult `json:"messages"`
	Truncated bool            `json:"truncated,omitempty"`
}

// SearchChannelsCommand lists Slack channels filtered by a substring name pattern
// and returns each channel together with its messages and resolved user names.
//
// Filtering defaults (both flags false):
//   - system notifications (channel_join, channel_leave, topic changes, …) hidden unless SystemEvents=true
//   - bot/app integration messages hidden unless BotMessages=true
type SearchChannelsCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
	SystemEvents  bool // include system notification messages
	BotMessages   bool // include bot/app integration messages
}

// matchedChannel holds a channel and its fetched messages during Run.
type matchedChannel struct {
	ch        slack.Channel
	messages  []slack.Message
	truncated bool
}

// Run calls conversations.list, filters channels whose names contain namePattern
// as a substring (case-insensitive; hyphens and spaces are treated as equivalent),
// fetches each matched channel's messages concurrently, resolves user IDs to display
// names, and writes a JSON array to Output. An empty result set is written as [].
func (c *SearchChannelsCommand) Run(ctx context.Context, workspace, namePattern string) error {
	if namePattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}

	client, err := resolveClient(ctx, c.Store, workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	channels, err := client.ListChannels(ctx)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: workspace}
		}
		return fmt.Errorf("listing channels: %w", err)
	}

	// Filter channels client-side (Slack's API has no server-side name filter).
	normPattern := normalizeChannelName(namePattern)
	var candidates []slack.Channel
	for _, ch := range channels {
		if strings.Contains(normalizeChannelName(ch.Name), normPattern) {
			candidates = append(candidates, ch)
		}
	}

	// Fetch messages for each candidate concurrently to reduce wall-clock time.
	// Each goroutine owns a unique index i, so concurrent writes to matched[i]
	// are safe without a mutex (different indices → different memory addresses).
	matched := make([]matchedChannel, len(candidates))
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentChannelFetches)
	for i, ch := range candidates {
		g.Go(func() error {
			messages, truncated, msgErr := client.GetChannelMessages(gCtx, ch.ID, 0)
			if msgErr != nil {
				if errors.Is(msgErr, slack.ErrNoChannelAccess) {
					matched[i] = matchedChannel{ch: ch}
					return nil
				}
				return fmt.Errorf("fetching messages for channel %q: %w", ch.Name, msgErr)
			}
			matched[i] = matchedChannel{ch: ch, messages: messages, truncated: truncated}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// nameMap may be nil when no messages have user IDs; reading a nil map returns
	// "" without panicking, so the fallback below handles it correctly.
	nameMap := resolveUsers(ctx, client, collectUserIDs(matched))

	results := make([]channelResult, 0, len(matched))
	for _, m := range matched {
		msgResults := make([]messageResult, 0, len(m.messages))
		for _, msg := range m.messages {
			if !c.SystemEvents && msg.IsSystemMessage() {
				continue
			}
			if !c.BotMessages && msg.IsBotMessage() {
				continue
			}
			user := nameMap[msg.UserID]
			if user == "" {
				user = msg.UserID
			}
			msgResults = append(msgResults, messageResult{
				User:      user,
				Text:      msg.Text,
				Timestamp: msg.Timestamp,
			})
		}
		results = append(results, channelResult{
			ID:        m.ch.ID,
			Name:      m.ch.Name,
			Messages:  msgResults,
			Truncated: m.truncated,
		})
	}

	return json.NewEncoder(c.Output).Encode(results)
}

// collectUserIDs returns the deduplicated set of non-empty user IDs found across
// all messages in matched, preserving first-seen order.
func collectUserIDs(matched []matchedChannel) []string {
	seen := make(map[string]struct{})
	var uids []string
	for _, m := range matched {
		for _, msg := range m.messages {
			if msg.UserID == "" {
				continue
			}
			if _, ok := seen[msg.UserID]; !ok {
				seen[msg.UserID] = struct{}{}
				uids = append(uids, msg.UserID)
			}
		}
	}
	return uids
}

// resolveUsers resolves a slice of Slack user IDs to display names concurrently,
// returning a userID → display name map. Resolution failures fall back to the raw
// user ID rather than propagating an error. Returns nil when uids is empty.
func resolveUsers(ctx context.Context, client *slack.Client, uids []string) map[string]string {
	if len(uids) == 0 {
		return nil
	}

	cache := slack.NewUserCache(client)
	nameMap := make(map[string]string, len(uids))
	var mu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentUserResolves)
	for _, uid := range uids {
		g.Go(func() error {
			name, resolveErr := cache.Resolve(gCtx, uid)
			if resolveErr != nil {
				name = uid
			}
			mu.Lock()
			nameMap[uid] = name
			mu.Unlock()
			return nil
		})
	}
	// Goroutines never return errors — resolution failures fall back to the raw UID above.
	_ = g.Wait()
	return nameMap
}

// normalizeChannelName lowercases s and replaces hyphens with spaces so that
// "epic-970" and "epic 970" compare equal during matching. Underscores are not
// normalised — "epic_970" and "epic-970" are treated as distinct.
func normalizeChannelName(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "-", " ")
}
