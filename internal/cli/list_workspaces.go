package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/artschekoff/slack-cli/internal/credentials"
)

// ListWorkspacesCommand lists all stored workspace names.
type ListWorkspacesCommand struct {
	Store  *credentials.Store
	Output io.Writer
}

// Run writes one workspace name per line to Output, with no additional decoration.
func (c *ListWorkspacesCommand) Run(ctx context.Context) error {
	names, err := c.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}
	for _, name := range names {
		fmt.Fprintln(c.Output, name)
	}
	return nil
}
