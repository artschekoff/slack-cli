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

// SearchCommand searches Slack messages for a given query.
type SearchCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run searches workspace for query and writes formatted results to Output.
func (c *SearchCommand) Run(ctx context.Context, workspace, query string, count int, startFrom time.Time) error {
	client, err := resolveClient(ctx, c.Store, workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	if count < 1 {
		count = 1
	} else if count > 100 {
		count = 100
	}

	matches, total, err := client.SearchMessages(ctx, query, count, startFrom)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: workspace}
		}
		return fmt.Errorf("%w: %v", ErrSlackSearch, err)
	}

	if len(matches) == 0 {
		fmt.Fprintf(c.Output, "No results found for query: %q", query)
		return nil
	}

	groups := slack.GroupByChannel(matches)
	fmt.Fprintf(c.Output, "Search results for %q in workspace '%s' (showing %d of %d total):\n\n",
		query, workspace, len(matches), total)
	for _, g := range groups {
		fmt.Fprintf(c.Output, "#%s (%s) — %d messages:\n", g.ChannelName, g.ChannelID, len(g.Messages))
		for i, m := range g.Messages {
			fmt.Fprintf(c.Output, "  %d. %s | %s\n     %s\n     Permalink: %s\n     thread_ts: %s\n\n",
				i+1, m.Username, m.Timestamp, m.Text, m.Permalink, searchPermalinkToTS(m.Permalink))
		}
	}
	return nil
}

// searchPermalinkToTS extracts thread_ts from a Slack permalink.
// Format: https://workspace.slack.com/archives/CXXXXXXXX/p1234567890123456
func searchPermalinkToTS(permalink string) string {
	parts := strings.Split(permalink, "/")
	for _, p := range parts {
		if len(p) < 11 || p[0] != 'p' {
			continue
		}
		digits := p[1:]
		if !searchIsDigits(digits) {
			continue
		}
		return digits[:10] + "." + digits[10:]
	}
	return ""
}

func searchIsDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

