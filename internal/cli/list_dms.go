package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"golang.org/x/sync/errgroup"
)

// dmResult is one entry in the JSON output of list-dms.
type dmResult struct {
	ID       string `json:"id"`
	UserID   string `json:"userId,omitempty"`
	UserName string `json:"userName,omitempty"`
	Name     string `json:"name,omitempty"`
	IsIM     bool   `json:"isIm"`
}

// ListDMsCommand lists Slack direct message conversations (1:1 and group DMs)
// and resolves user IDs to display names for 1:1 DMs.
type ListDMsCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run fetches DM conversations via conversations.list, resolves user IDs for
// 1:1 DMs concurrently, and writes a JSON array to Output. When startFrom is
// non-zero, only conversations created on or after that date are included.
func (c *ListDMsCommand) Run(ctx context.Context, workspace string, startFrom time.Time) error {
	client, err := resolveClient(ctx, c.Store, workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	dms, err := client.ListDMs(ctx, startFrom)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: workspace}
		}
		return fmt.Errorf("listing DMs: %w", err)
	}

	nameMap := resolveDMUsers(ctx, client, dms)

	results := make([]dmResult, 0, len(dms))
	for _, dm := range dms {
		r := dmResult{
			ID:   dm.ID,
			IsIM: dm.IsIM,
		}
		if dm.IsIM {
			r.UserID = dm.UserID
			name := nameMap[dm.UserID]
			if name == "" {
				name = dm.UserID
			}
			r.UserName = name
		} else {
			r.Name = dm.Name
		}
		results = append(results, r)
	}

	return json.NewEncoder(c.Output).Encode(results)
}

// resolveDMUsers resolves the unique set of user IDs from 1:1 DMs to display names.
func resolveDMUsers(ctx context.Context, client *slack.Client, dms []slack.DM) map[string]string {
	seen := make(map[string]struct{})
	var uids []string
	for _, dm := range dms {
		if !dm.IsIM || dm.UserID == "" {
			continue
		}
		if _, ok := seen[dm.UserID]; !ok {
			seen[dm.UserID] = struct{}{}
			uids = append(uids, dm.UserID)
		}
	}
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
	_ = g.Wait()
	return nameMap
}
