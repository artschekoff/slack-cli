package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// LoadThreadCommand loads all messages in a Slack thread.
type LoadThreadCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run fetches thread replies and writes them formatted to Output.
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

	if len(messages) == 0 {
		fmt.Fprint(c.Output, "No messages found in this thread.")
		return nil
	}

	fmt.Fprintf(c.Output, "Thread (%d messages):\n\n", len(messages))
	for _, m := range messages {
		reactions := threadFormatReactions(m.Reactions)
		files := threadFormatFiles(m.Files)

		fmt.Fprintf(c.Output, "**%s** (%s):\n%s\n", m.UserID, m.Timestamp, m.Text)
		if reactions != "" {
			fmt.Fprintf(c.Output, "Reactions: %s\n", reactions)
		}
		if files != "" {
			fmt.Fprintf(c.Output, "Attachments: %s\n", files)
		}
		fmt.Fprint(c.Output, "\n---\n\n")
	}

	if truncated {
		fmt.Fprint(c.Output, "⚠️ Results truncated — thread has more messages than the pagination limit allows.\n")
	}
	return nil
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
