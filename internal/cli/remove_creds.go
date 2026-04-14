package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/artschekoff/slack-cli/internal/credentials"
)

// RemoveCredsCommand removes a workspace's credentials from the store.
type RemoveCredsCommand struct {
	Store  *credentials.Store
	Input  io.Reader
	Output io.Writer
}

// Run removes credentials for the given workspace. If workspace is empty,
// the user is prompted for a name. Returns an error if the workspace is not found.
func (c *RemoveCredsCommand) Run(ctx context.Context, workspace string) error {
	if workspace == "" {
		scanner := bufio.NewScanner(c.Input)
		fmt.Fprint(c.Output, "Workspace to remove: ")
		if !scanner.Scan() {
			return errors.New("workspace name is required")
		}
		workspace = strings.TrimSpace(scanner.Text())
		if workspace == "" {
			return errors.New("workspace name is required")
		}
	}

	if err := credentials.ValidateWorkspaceName(workspace); err != nil {
		return err
	}

	if err := c.Store.Delete(ctx, workspace); err != nil {
		if errors.Is(err, credentials.ErrWorkspaceNotFound) {
			return fmt.Errorf("workspace %q not found", workspace)
		}
		return fmt.Errorf("removing credentials: %w", err)
	}

	fmt.Fprintf(c.Output, "Credentials for workspace %q removed.\n", workspace)
	return nil
}
