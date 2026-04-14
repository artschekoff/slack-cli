package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/artschekoff/slack-cli/internal/credentials"
)

// ListWorkspacesCommand lists all stored workspace names.
type ListWorkspacesCommand struct {
	Store  *credentials.Store
	Output io.Writer
}

// Run writes workspace names to Output or a "none found" message.
func (c *ListWorkspacesCommand) Run(ctx context.Context) error {
	names, err := c.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}
	if len(names) == 0 {
		fmt.Fprint(c.Output, "No workspaces found. Use auth_start to authenticate with a workspace.")
		return nil
	}
	fmt.Fprintf(c.Output, "Saved workspaces:\n- %s", strings.Join(names, "\n- "))
	return nil
}
