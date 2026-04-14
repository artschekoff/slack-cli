package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/artschekoff/slack-cli/internal/credentials"
)

// GetCredentialsCommand checks whether credentials exist for a workspace (without exposing secrets).
type GetCredentialsCommand struct {
	Store  *credentials.Store
	Output io.Writer
}

// Run writes credential presence status for workspace to Output.
func (c *GetCredentialsCommand) Run(ctx context.Context, workspace string) error {
	creds, err := c.Store.Get(ctx, workspace)
	if errors.Is(err, credentials.ErrWorkspaceNotFound) {
		fmt.Fprintf(c.Output, "No credentials found for workspace '%s'. Use auth_start to authenticate.", workspace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading credentials: %w", err)
	}
	fmt.Fprintf(c.Output, "Workspace '%s' credentials:\n- token: %s\n- cookie: %s",
		workspace, credStatus(creds.Token != ""), credStatus(creds.Cookie != ""))
	return nil
}

func credStatus(present bool) string {
	if present {
		return "present"
	}
	return "missing"
}
