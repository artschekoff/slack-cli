package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"golang.org/x/sync/errgroup"
)

const maxConcurrentUserResolvesCtx = 5

// LoadContextArgs holds all parameters for LoadContextCommand.Run.
type LoadContextArgs struct {
	Workspace   string
	ChannelID   string
	ThreadTS    string
	Permalink   string
	ChannelName string
	SearchQuery string
	StartFrom   time.Time
}

// LoadContextCommand loads a Slack thread, resolves user names, and writes a
// formatted markdown context block ready for AI consumption.
type LoadContextCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run fetches the thread, resolves users concurrently, and writes markdown output.
func (c *LoadContextCommand) Run(ctx context.Context, args LoadContextArgs) error {
	client, err := resolveClient(ctx, c.Store, args.Workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	messages, truncated, err := client.GetThreadReplies(ctx, args.ChannelID, args.ThreadTS, args.StartFrom)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: args.Workspace}
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

	userCache := slack.NewUserCache(client)
	uniqueUsers := ctxCollectUniqueUsers(messages)

	nameMap := make(map[string]string, len(uniqueUsers))
	var mu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentUserResolvesCtx)
	for _, uid := range uniqueUsers {
		uid := uid
		g.Go(func() error {
			name, resolveErr := userCache.Resolve(gCtx, uid)
			if resolveErr != nil {
				name = uid
			}
			mu.Lock()
			nameMap[uid] = name
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	displayChannel := args.ChannelName
	if displayChannel == "" {
		displayChannel = args.ChannelID
	}

	startDate := messages[0].Timestamp
	label := args.SearchQuery
	if label == "" {
		label = args.ThreadTS
	}

	fmt.Fprintf(c.Output, "# Slack Context: %q — #%s\n\n", label, displayChannel)
	fmt.Fprintf(c.Output, "**Source:** #%s | **Started:** %s | **Messages:** %d\n",
		displayChannel, startDate, len(messages))
	if args.Permalink != "" {
		fmt.Fprintf(c.Output, "**Permalink:** %s\n", args.Permalink)
	}
	fmt.Fprint(c.Output, "\n---\n\n")

	for _, m := range messages {
		author := nameMap[m.UserID]
		if author == "" {
			author = m.UserID
		}
		fmt.Fprintf(c.Output, "> **%s** (%s):\n> %s\n",
			author, m.Timestamp, ctxFormatMessageText(m.Text))
		if len(m.Reactions) > 0 {
			fmt.Fprintf(c.Output, "> _Reactions: %s_\n", threadFormatReactions(m.Reactions))
		}
		if len(m.Files) > 0 {
			fmt.Fprintf(c.Output, "> _Attachments: %s_\n", threadFormatFiles(m.Files))
		}
		fmt.Fprint(c.Output, "\n---\n\n")
	}

	fmt.Fprintf(c.Output, "Slack context loaded: **%d** message(s) from **#%s** (thread: %s). This conversation is now in context.",
		len(messages), displayChannel, args.ThreadTS)
	if truncated {
		fmt.Fprint(c.Output, "\n\n⚠️ Results truncated — thread has more messages than the pagination limit allows.")
	}
	return nil
}

func ctxCollectUniqueUsers(messages []slack.Message) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(messages))
	for _, m := range messages {
		if m.UserID == "" {
			continue
		}
		if _, ok := seen[m.UserID]; !ok {
			seen[m.UserID] = struct{}{}
			result = append(result, m.UserID)
		}
	}
	return result
}

func ctxFormatMessageText(text string) string {
	lines := strings.Split(text, "\n")
	return strings.Join(lines, "\n> ")
}

