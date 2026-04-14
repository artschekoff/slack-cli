package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// AuthCompleteCommand validates and saves Slack credentials non-interactively.
// Unlike the interactive AuthCommand, it accepts workspace, token, and cookie as arguments.
type AuthCompleteCommand struct {
	Store    *credentials.Store
	Output   io.Writer
	Validate TokenValidatorFunc
}

// Run validates the given token and cookie, then saves them for the workspace.
func (c *AuthCompleteCommand) Run(ctx context.Context, workspace, token, cookie string) error {
	if err := credentials.ValidateWorkspaceName(workspace); err != nil {
		return err
	}
	if !strings.HasPrefix(token, "xoxc-") {
		return fmt.Errorf("invalid token: must start with 'xoxc-'")
	}
	if !strings.HasPrefix(cookie, "xoxd-") {
		return fmt.Errorf("invalid cookie: must start with 'xoxd-'")
	}

	result, err := c.Validate(ctx, token, cookie)
	if err != nil {
		if errors.Is(err, slack.ErrUnauthorized) {
			return fmt.Errorf("credentials are invalid or expired. Please re-run auth-start to get fresh credentials")
		}
		return fmt.Errorf("validating credentials: %w", err)
	}

	if err := c.Store.Save(ctx, workspace, credentials.Creds{Token: token, Cookie: cookie}); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Fprintf(c.Output, "Authentication successful!\n- Workspace: %s (%s)\n- User: %s (%s)\n- Credentials saved.",
		result.Team, result.TeamID, result.User, result.UserID)
	return nil
}
