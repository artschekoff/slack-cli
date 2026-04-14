package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// TestCredsCommand checks whether stored credentials for a workspace are
// present and valid. It decrypts credentials (if encrypted) and validates
// them against the Slack API without modifying the store.
type TestCredsCommand struct {
	Store    *credentials.Store
	Input    io.Reader
	Output   io.Writer
	Validate TokenValidatorFunc
}

// Run checks credentials for the given workspace. If workspace is empty, the
// user is prompted. Reports validity status without leaking secrets.
func (c *TestCredsCommand) Run(ctx context.Context, workspace string) error {
	if workspace == "" {
		scanner := bufio.NewScanner(c.Input)
		fmt.Fprint(c.Output, "Workspace name (e.g. acme, globex-corp): ")
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

	creds, err := c.Store.Get(ctx, workspace)
	if errors.Is(err, credentials.ErrWorkspaceNotFound) {
		fmt.Fprintf(c.Output, "Workspace %q: no credentials found.\n", workspace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading stored credentials: %w", err)
	}

	result, err := c.Validate(ctx, creds.Token, creds.Cookie)
	if errors.Is(err, slack.ErrUnauthorized) {
		fmt.Fprintf(c.Output, "Workspace %q: credentials are invalid (expired or revoked).\n", workspace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("validating credentials: %w", err)
	}

	fmt.Fprintf(c.Output, "Workspace %q: credentials are valid.\n", workspace)
	fmt.Fprintf(c.Output, "  Workspace: %s (%s)\n", result.Team, result.TeamID)
	fmt.Fprintf(c.Output, "  User:      %s (%s)\n", result.User, result.UserID)
	return nil
}
