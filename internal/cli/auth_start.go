package cli

import (
	"context"
	"fmt"
	"io"
)

// AuthStartCommand opens Slack in the browser and prints credential extraction instructions.
type AuthStartCommand struct {
	OpenBrowser func(url string) error
	Output      io.Writer
}

// Run opens the browser and prints step-by-step instructions to Output.
func (c *AuthStartCommand) Run(_ context.Context, workspace string) error {
	if err := c.OpenBrowser(slackAppURL); err != nil {
		fmt.Fprintf(c.Output, "Warning: could not open browser: %v\nPlease open %s manually.\n\n", err, slackAppURL)
	}

	jsCmd := bold + `JSON.parse(localStorage.localConfig_v2).teams[Object.keys(JSON.parse(localStorage.localConfig_v2).teams)[0]].token` + reset

	wsNote := ""
	if workspace == "" {
		wsNote = "\n\nYou haven't specified a workspace yet. Once you have both values, run:\n  slack-cli auth-complete <workspace> --token xoxc-... --cookie xoxd-..."
	} else {
		wsNote = fmt.Sprintf("\n\nOnce you have both values, run:\n  slack-cli auth-complete %s --token xoxc-... --cookie xoxd-...", workspace)
	}

	fmt.Fprintf(c.Output, `Slack authentication — extract credentials from your browser.

Step 1 — Extract the token:
  1. Log in to Slack at %s if needed
  2. Open DevTools (Cmd+Option+I on Mac, F12 on Windows)
  3. Go to the Console tab
  4. Run this command:
     %s
  5. Copy the value starting with xoxc-

Step 2 — Extract the cookie:
  1. In DevTools, go to Application → Cookies → https://app.slack.com
  2. Find the cookie named %s
  3. Copy the value starting with xoxd-%s
`, slackAppURL, jsCmd, bold+"d"+reset, wsNote)
	return nil
}
