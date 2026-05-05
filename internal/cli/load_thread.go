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

// LoadThreadMessage is one message in a LoadThreadResult.
type LoadThreadMessage struct {
	UserID    string `json:"userId"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Reactions string `json:"reactions,omitempty"`
	Files     string `json:"files,omitempty"`
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
		out.Messages = append(out.Messages, LoadThreadMessage{
			UserID:    m.UserID,
			Timestamp: m.Timestamp,
			Text:      m.Text,
			Reactions: threadFormatReactions(m.Reactions),
			Files:     threadFormatFiles(m.Files),
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
