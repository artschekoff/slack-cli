package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// ReactionSummary is one reaction on a message.
type ReactionSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// LoadThreadMessage is one message in a LoadThreadResult.
type LoadThreadMessage struct {
	UserID    string            `json:"userId"`
	Timestamp string            `json:"timestamp"`
	Text      string            `json:"text"`
	Reactions []ReactionSummary `json:"reactions,omitempty"`
	Files     []string          `json:"files,omitempty"`
}

// LoadThreadResult is the JSON output of load-thread.
type LoadThreadResult struct {
	Messages  []LoadThreadMessage `json:"messages"`
	Truncated bool                `json:"truncated"`
}

// LoadThreadCommand loads all messages in a Slack thread.
type LoadThreadCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run fetches thread replies and writes them as JSON to Output.
func (c *LoadThreadCommand) Run(ctx context.Context, workspace, channelID, threadTS string, startFrom time.Time) error {
	client, err := resolveClient(ctx, c.Store, workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	messages, truncated, err := client.GetThreadReplies(ctx, channelID, threadTS, startFrom)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: workspace}
		}
		if errors.Is(err, slack.ErrNoChannelAccess) {
			return &noChannelAccessError{}
		}
		return fmt.Errorf("%w: %v", ErrSlackLoadThread, err)
	}

	out := LoadThreadResult{
		Messages:  make([]LoadThreadMessage, 0, len(messages)),
		Truncated: truncated,
	}
	for _, m := range messages {
		reactions := make([]ReactionSummary, 0, len(m.Reactions))
		for _, r := range m.Reactions {
			reactions = append(reactions, ReactionSummary{Name: r.Name, Count: r.Count})
		}
		out.Messages = append(out.Messages, LoadThreadMessage{
			UserID:    m.UserID,
			Timestamp: m.RawTS,
			Text:      m.Text,
			Reactions: reactions,
			Files:     m.Files,
		})
	}

	return json.NewEncoder(c.Output).Encode(out)
}

func threadFormatReactions(reactions []slack.Reaction) string {
	if len(reactions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reactions))
	for _, r := range reactions {
		parts = append(parts, fmt.Sprintf(":%s: %d", r.Name, r.Count))
	}
	return strings.Join(parts, ", ")
}

func threadFormatFiles(files []string) string {
	return strings.Join(files, ", ")
}
