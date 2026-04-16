package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// GetUserCommand resolves a Slack user ID to a display name.
type GetUserCommand struct {
	Store         *credentials.Store
	Output        io.Writer
	ClientFactory ClientFactory
}

// Run resolves userID for workspace and writes the result to Output.
func (c *GetUserCommand) Run(ctx context.Context, workspace, userID string) error {
	client, err := resolveClient(ctx, c.Store, workspace, c.ClientFactory)
	if err != nil {
		return err
	}

	name, err := client.GetUserInfo(ctx, userID)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return &unauthorizedError{workspace: workspace}
		}
		return fmt.Errorf("%w: %v", ErrSlackGetUser, err)
	}

	fmt.Fprintf(c.Output, "User %s: %s", userID, name)
	return nil
}
