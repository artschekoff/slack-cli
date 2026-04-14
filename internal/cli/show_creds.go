package cli

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/artschekoff/slack-cli/internal/credentials"
)

// ShowCredsCommand prints all stored workspace names (without secrets) and the path to the credentials file.
type ShowCredsCommand struct {
	Store  *credentials.Store
	Output io.Writer
}

// Run lists all workspaces stored in the credentials file and prints the file path.
func (c *ShowCredsCommand) Run(ctx context.Context) error {
	names, err := c.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}

	fmt.Fprintf(c.Output, "Credentials file: %s\n\n", c.Store.Path())

	if len(names) == 0 {
		fmt.Fprintln(c.Output, "No workspaces saved.")
		return nil
	}

	sort.Strings(names)

	fmt.Fprintf(c.Output, "Saved workspaces (%d):\n", len(names))
	for _, name := range names {
		fmt.Fprintf(c.Output, "  • %s\n", name)
	}

	return nil
}
