// Package cli implements CLI commands for slack-cli.
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

const slackAppURL = "https://app.slack.com"

// TokenValidatorFunc validates a Slack token+cookie pair and returns auth info.
type TokenValidatorFunc func(ctx context.Context, token, cookie string) (slack.AuthTestResult, error)

// DefaultValidator builds a real Slack client and calls auth.test.
func DefaultValidator(ctx context.Context, token, cookie string) (slack.AuthTestResult, error) {
	return slack.NewClient(token, cookie).ValidateToken(ctx)
}

// AuthCommand runs the interactive auth flow: open browser, print
// extraction instructions, read token+cookie from stdin, validate, and save.
type AuthCommand struct {
	Store       *credentials.Store
	OpenBrowser func(url string) error
	Input       io.Reader
	Output      io.Writer
	Validate    TokenValidatorFunc
}

// Run executes the auth flow. If workspace is empty, it prompts the user.
// If valid credentials for the workspace are already stored, they are reused
// without going through the full browser-based flow. If stored credentials
// have expired, the user is informed and the full flow runs to re-authenticate.
func (c *AuthCommand) Run(ctx context.Context, workspace string) error {
	scanner := bufio.NewScanner(c.Input)

	if workspace == "" {
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

	if reused, err := c.tryExistingCredentials(ctx, workspace); err != nil {
		return err
	} else if reused {
		return nil
	}

	if err := c.OpenBrowser(slackAppURL); err != nil {
		fmt.Fprintf(c.Output, "Warning: could not open browser: %v\n", err)
		fmt.Fprintf(c.Output, "Please open %s manually.\n\n", slackAppURL)
	}

	c.printInstructions()

	fmt.Fprint(c.Output, "Token (xoxc-...): ")
	if !scanner.Scan() {
		return errors.New("token input is required")
	}
	token := stripQuotes(strings.TrimSpace(scanner.Text()))
	if !strings.HasPrefix(token, "xoxc-") {
		return errors.New("invalid token: must start with 'xoxc-'")
	}

	fmt.Fprint(c.Output, "Cookie (xoxd-...): ")
	if !scanner.Scan() {
		return errors.New("cookie input is required")
	}
	cookie := stripQuotes(strings.TrimSpace(scanner.Text()))
	if !strings.HasPrefix(cookie, "xoxd-") {
		return errors.New("invalid cookie: must start with 'xoxd-'")
	}

	fmt.Fprint(c.Output, "\nValidating credentials...")
	result, err := c.Validate(ctx, token, cookie)
	if err != nil {
		fmt.Fprintln(c.Output, " failed.")
		return fmt.Errorf("credential validation: %w", err)
	}
	fmt.Fprintln(c.Output, " ok.")

	if err := c.Store.Save(ctx, workspace, credentials.Creds{Token: token, Cookie: cookie}); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Fprintf(c.Output, "\nAuthentication successful!\n")
	fmt.Fprintf(c.Output, "  Workspace: %s (%s)\n", result.Team, result.TeamID)
	fmt.Fprintf(c.Output, "  User:      %s (%s)\n", result.User, result.UserID)
	fmt.Fprintf(c.Output, "  Credentials saved for workspace %q.\n", workspace)

	return nil
}

// tryExistingCredentials checks whether the workspace already has stored
// credentials and, if so, validates them. Returns (true, nil) when the
// stored credentials are still valid and the caller can skip the full auth
// flow. Returns (false, nil) when no credentials exist or they are expired
// (ErrUnauthorized), letting the caller continue with the browser-based flow.
// Non-auth errors (network failures, timeouts) are propagated to the caller.
func (c *AuthCommand) tryExistingCredentials(ctx context.Context, workspace string) (bool, error) {
	creds, err := c.Store.Get(ctx, workspace)
	if errors.Is(err, credentials.ErrWorkspaceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading stored credentials: %w", err)
	}

	result, err := c.Validate(ctx, creds.Token, creds.Cookie)
	if errors.Is(err, slack.ErrUnauthorized) {
		fmt.Fprintf(c.Output, "Stored credentials for %q have expired. Re-authenticating...\n\n", workspace)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validating stored credentials: %w", err)
	}

	fmt.Fprintf(c.Output, "Credentials already valid for workspace %q.\n", workspace)
	fmt.Fprintf(c.Output, "  Workspace: %s (%s)\n", result.Team, result.TeamID)
	fmt.Fprintf(c.Output, "  User:      %s (%s)\n", result.User, result.UserID)
	return true, nil
}

const (
	bold  = "\033[1m"
	reset = "\033[0m"
)

func (c *AuthCommand) printInstructions() {
	jsCmd := bold + `JSON.parse(localStorage.localConfig_v2).teams[Object.keys(JSON.parse(localStorage.localConfig_v2).teams)[0]].token` + reset

	fmt.Fprintf(c.Output, `
Slack authentication — extract credentials from your browser.

Step 1 — Extract the token:
  1. Log in to Slack at https://app.slack.com if needed
  2. Open DevTools (Cmd+Option+I on Mac, F12 on Windows)
  3. Go to the Console tab
  4. Run this command:
     %s
  5. Copy the value starting with xoxc-

Step 2 — Extract the cookie:
  1. In DevTools, go to Application → Cookies → https://app.slack.com
  2. Find the cookie named %s
  3. Copy the value starting with xoxd-

`, jsCmd, bold+"d"+reset)
}

// stripQuotes removes matching surrounding single or double quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
