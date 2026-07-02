package cli

import (
	"io"
	"os"
	"os/signal"

	"github.com/artschekoff/slack-cli/internal/browser"
	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/spf13/cobra"
)

// RootDeps holds all injectable dependencies for the Cobra command tree.
// Providing these as a struct keeps each subcommand fully testable without
// touching the filesystem or a real browser.
type RootDeps struct {
	Store         *credentials.Store
	OpenBrowser   func(url string) error
	Input         io.Reader
	Output        io.Writer
	Validate      TokenValidatorFunc
	ClientFactory ClientFactory
}

// DefaultRootDeps builds production-ready dependencies.
func DefaultRootDeps(store *credentials.Store) RootDeps {
	return RootDeps{
		Store:         store,
		OpenBrowser:   browser.Open,
		Input:         os.Stdin,
		Output:        os.Stdout,
		Validate:      DefaultValidator,
		ClientFactory: DefaultClientFactory(),
	}
}

// NewRootCmd returns the top-level cobra.Command for slack-cli.
func NewRootCmd(deps RootDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "slack-cli",
		Short: "Slack CLI — credential management and Slack operations",
		Long: `slack-cli — manage Slack credentials and interact with Slack from the terminal.

Credential management:
  slack-cli auth [workspace]                    interactive auth (browser + prompts)
  slack-cli auth-start [workspace]              print browser extraction instructions
  slack-cli auth-complete <workspace>           save credentials non-interactively (--token, --cookie)
  slack-cli list-workspaces                     list saved workspace names (plain text, one per line)
  slack-cli get-credentials <workspace>         check whether credentials exist (plain text)
  slack-cli show-creds                          show workspace list and credentials file path (plain text)
  slack-cli test-creds [workspace]              validate stored credentials against Slack API (plain text)
  slack-cli remove-creds [workspace]            delete stored credentials

Slack operations:
  slack-cli search <workspace> <query>                  full-text message search (plain text, --count, --start-from)
  slack-cli search-channels <workspace> <pattern>       find channels by name, return messages (JSON array)
  slack-cli list-dms <workspace>                        list DM conversations with resolved names (JSON array)
  slack-cli load-thread <workspace> <ch> <ts>           fetch all thread messages (JSON object, --start-from)
  slack-cli load-context <workspace> <ch> <ts>          fetch thread as markdown with resolved names (markdown)
  slack-cli get-user <workspace> <user-id>              resolve a Slack user ID to display name (plain text)

Use "slack-cli <command> --help" for full argument, flag, and output format details.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newAuthCmd(deps))
	root.AddCommand(newShowCredsCmd(deps))
	root.AddCommand(newRemoveCredsCmd(deps))
	root.AddCommand(newTestCredsCmd(deps))
	root.AddCommand(newListWorkspacesCmd(deps))
	root.AddCommand(newGetCredentialsCmd(deps))
	root.AddCommand(newAuthStartCmd(deps))
	root.AddCommand(newAuthCompleteCmd(deps))
	root.AddCommand(newSearchCmd(deps))
	root.AddCommand(newSearchChannelsCmd(deps))
	root.AddCommand(newListDMsCmd(deps))
	root.AddCommand(newLoadThreadCmd(deps))
	root.AddCommand(newGetUserCmd(deps))
	root.AddCommand(newLoadContextCmd(deps))

	return root
}

func newAuthCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "auth [workspace]",
		Short: "Authenticate with a Slack workspace",
		Long: `Opens Slack in your browser and guides you through extracting the
session token and cookie. If credentials for the workspace are already
stored and still valid, they are reused without re-authenticating.

Input:
  workspace  Optional name to assign to this workspace (prompted if omitted)

Output: plain text status messages confirming successful save or reporting errors.

Example:
  slack-cli auth acme`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) > 0 {
				workspace = args[0]
			}
			cmd := &AuthCommand{
				Store:       deps.Store,
				OpenBrowser: deps.OpenBrowser,
				Input:       deps.Input,
				Output:      deps.Output,
				Validate:    deps.Validate,
			}
			ctx, stop := signal.NotifyContext(cobraCmd.Context(), os.Interrupt)
			defer stop()
			return cmd.Run(ctx, workspace)
		},
	}
}

func newShowCredsCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show-creds",
		Short: "List saved workspaces and show the credentials file path",
		Long: `Prints all saved workspace names (without secrets) and the path
to ~/.slack/workspace_credentials.json.

Output: plain text — one workspace name per line followed by the credentials file path.

Example:
  slack-cli show-creds`,
		Args: cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			cmd := &ShowCredsCommand{
				Store:  deps.Store,
				Output: deps.Output,
			}
			return cmd.Run(cobraCmd.Context())
		},
	}
}

func newRemoveCredsCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-creds [workspace]",
		Short: "Remove stored credentials for a workspace",
		Long: `Removes the saved token and cookie for the given workspace from
~/.slack/workspace_credentials.json. If no workspace is given, you will be prompted.

Input:
  workspace  Name of the workspace to remove (prompted if omitted)

Output: plain text confirmation of deletion.

Example:
  slack-cli remove-creds acme`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) > 0 {
				workspace = args[0]
			}
			cmd := &RemoveCredsCommand{
				Store:  deps.Store,
				Input:  deps.Input,
				Output: deps.Output,
			}
			return cmd.Run(cobraCmd.Context(), workspace)
		},
	}
}

func newTestCredsCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "test-creds [workspace]",
		Short: "Test whether stored credentials for a workspace are valid",
		Long: `Decrypts and validates the stored Slack credentials for the given
workspace by calling Slack's auth.test API. Reports whether the credentials are
present and valid without modifying the store. If no workspace is given, you will be prompted.

Input:
  workspace  Name of the workspace to test (prompted if omitted)

Output: plain text — reports "valid" with the authenticated user/team, or an error if invalid/missing.

Example:
  slack-cli test-creds acme`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) > 0 {
				workspace = args[0]
			}
			cmd := &TestCredsCommand{
				Store:    deps.Store,
				Input:    deps.Input,
				Output:   deps.Output,
				Validate: deps.Validate,
			}
			ctx, stop := signal.NotifyContext(cobraCmd.Context(), os.Interrupt)
			defer stop()
			return cmd.Run(ctx, workspace)
		},
	}
}

func newListWorkspacesCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list-workspaces",
		Short: "List all saved workspace names",
		Long: `Prints the names of all workspaces that have credentials stored in
~/.slack/workspace_credentials.json. No secrets are shown.

Output: plain text — one workspace name per line. Use these names as the <workspace> argument
for all other commands (search, load-thread, etc.).

Example:
  slack-cli list-workspaces`,
		Args: cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			cmd := &ListWorkspacesCommand{Store: deps.Store, Output: deps.Output}
			return cmd.Run(cobraCmd.Context())
		},
	}
}

func newGetCredentialsCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "get-credentials <workspace>",
		Short: "Check whether credentials exist for a workspace",
		Long: `Reports whether a token and cookie are stored for the given workspace.
The actual secret values are never printed; only "present" or "missing" is shown.

Input:
  workspace  Name of the saved workspace (e.g. acme)

Output: plain text — two lines reporting "token: present/missing" and "cookie: present/missing".

Example:
  slack-cli get-credentials acme`,
		Args: cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cmd := &GetCredentialsCommand{Store: deps.Store, Output: deps.Output}
			return cmd.Run(cobraCmd.Context(), args[0])
		},
	}
}

func newAuthStartCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "auth-start [workspace]",
		Short: "Open Slack in browser and print token/cookie extraction instructions",
		Long: `Opens https://app.slack.com in your browser and prints step-by-step
instructions for extracting your Slack session token (xoxc-...) and cookie
(xoxd-...) from the browser DevTools Network tab.

Input:
  workspace  Optional workspace name to use later with auth-complete (for reference only)

Output: plain text instructions printed to stdout. Nothing is saved yet.

Once you have both values, complete authentication with:
  slack-cli auth-complete <workspace> --token xoxc-... --cookie xoxd-...

Examples:
  slack-cli auth-start
  slack-cli auth-start acme`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) > 0 {
				workspace = args[0]
			}
			cmd := &AuthStartCommand{OpenBrowser: deps.OpenBrowser, Output: deps.Output}
			return cmd.Run(cobraCmd.Context(), workspace)
		},
	}
}

func newAuthCompleteCmd(deps RootDeps) *cobra.Command {
	var token, cookie string
	c := &cobra.Command{
		Use:   "auth-complete <workspace>",
		Short: "Validate and save Slack credentials non-interactively",
		Long: `Validates the given token and cookie against Slack's auth.test API,
then saves them encrypted under the given workspace name. Unlike "auth", this
command is fully non-interactive and suitable for scripting or CI.

Input:
  workspace   Name to save credentials under (e.g. acme)
  --token     Slack user token (required); must start with "xoxc-"
  --cookie    Slack browser cookie "d" value (required); must start with "xoxd-"

Output: plain text confirmation that credentials were validated and saved.

Example:
  slack-cli auth-complete acme \
    --token xoxc-12345... \
    --cookie xoxd-abcde...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cmd := &AuthCompleteCommand{
				Store:    deps.Store,
				Output:   deps.Output,
				Validate: deps.Validate,
			}
			ctx, stop := signal.NotifyContext(cobraCmd.Context(), os.Interrupt)
			defer stop()
			return cmd.Run(ctx, args[0], token, cookie)
		},
	}
	c.Flags().StringVar(&token, "token", "", "Slack user token starting with 'xoxc-'")
	c.Flags().StringVar(&cookie, "cookie", "", "Slack browser cookie 'd' value starting with 'xoxd-'")
	return c
}

func newSearchCmd(deps RootDeps) *cobra.Command {
	var count int
	var startFrom string
	c := &cobra.Command{
		Use:   "search <workspace> <query>",
		Short: "Search Slack messages",
		Long: `Searches Slack messages using the full-text search API (same syntax as Slack's
search bar) and prints matching results grouped by channel.

Input:
  workspace  Name of the saved workspace (e.g. acme)
  query      Free-text search query (e.g. "deployment failed", "in:#general bug")

Flags:
  --count       Maximum number of results to return (default 20, max 100)
  --start-from  Only include messages on or after this date (YYYY-MM-DD)

Output format (plain text, stdout):
  Search results for "query" in workspace 'acme' (showing N of M total):

  #channel-name (CXXXXXXXX) — N messages:
    1. Username | 2024-01-15 10:30 | Message text here
       Permalink: https://acme.slack.com/archives/C.../p...
       thread_ts: 1705312200.000000

  Use thread_ts + channel ID with load-thread or load-context to fetch the full thread.

Examples:
  slack-cli search acme "deployment failed"
  slack-cli search acme "bug" --count 50
  slack-cli search acme "release" --start-from 2024-01-01`,
		Args: cobra.ExactArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			sf, err := ParseStartFrom(startFrom)
			if err != nil {
				return err
			}
			cmd := &SearchCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
			}
			return cmd.Run(cobraCmd.Context(), args[0], args[1], count, sf)
		},
	}
	c.Flags().IntVar(&count, "count", 20, "Maximum number of results (default 20, max 100)")
	c.Flags().StringVar(&startFrom, "start-from", "", "Only messages on or after this date (YYYY-MM-DD)")
	return c
}

func newSearchChannelsCmd(deps RootDeps) *cobra.Command {
	var systemEvents, botMessages bool
	c := &cobra.Command{
		Use:   "search-channels <workspace> <pattern>",
		Short: "List Slack channels matching a name pattern with their messages (JSON output)",
		Long: `Lists all accessible Slack channels whose names contain the given substring,
fetches their messages with thread replies, resolves user IDs to display names,
and writes a JSON array to stdout.

Matching is case-insensitive and treats hyphens as spaces, so "epic 970" and
"970" both match a channel named "epic-970". An empty result set is written as [].

By default system notifications (joined/left, topic changes, etc.) and bot/app
messages are filtered out.

Input:
  workspace  Name of the saved workspace (e.g. acme)
  pattern    Substring to match against channel names (case-insensitive, hyphens = spaces)

Flags:
  --system-events  Include system notification messages (channel_join, channel_leave, etc.)
  --bot-messages   Include bot and app integration messages

Output format (JSON array, stdout):
  [
    {
      "id": "CXXXXXXXX",
      "name": "epic-970",
      "truncated": false,
      "messages": [
        {
          "user": "Jane Doe",
          "text": "message text",
          "timestamp": "2024-01-15 10:30",
          "rawTs": "1705312200.000000",
          "replies": [
            {
              "user": "John Smith",
              "text": "reply text",
              "timestamp": "2024-01-15 10:31",
              "rawTs": "1705312260.000000"
            }
          ]
        }
      ]
    }
  ]

  rawTs is the raw Slack timestamp — use it as thread-ts with load-thread or load-context.
  replies is omitted when a message has no thread replies.
  truncated is true when the channel has more messages than the page limit allows.

Examples:
  slack-cli search-channels acme 970
  slack-cli search-channels acme "epic 970"
  slack-cli search-channels acme deploy --system-events
  slack-cli search-channels acme deploy --bot-messages`,
		Args: cobra.ExactArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cmd := &SearchChannelsCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
				SystemEvents:  systemEvents,
				BotMessages:   botMessages,
			}
			return cmd.Run(cobraCmd.Context(), args[0], args[1])
		},
	}
	c.Flags().BoolVar(&systemEvents, "system-events", false, "Include system notification messages (channel_join, channel_leave, etc.)")
	c.Flags().BoolVar(&botMessages, "bot-messages", false, "Include bot and app integration messages")
	return c
}

func newListDMsCmd(deps RootDeps) *cobra.Command {
	var startFrom string
	var withMessages, systemEvents bool
	c := &cobra.Command{
		Use:   "list-dms <workspace>",
		Short: "List direct message conversations with resolved user names (JSON output)",
		Long: `Lists all accessible Slack direct message conversations (1:1 and group DMs).
For 1:1 DMs, the other user's ID is resolved to a display name. Group DMs include
the auto-generated conversation name. Writes a JSON array to stdout.

When --start-from is set, only DMs that have a qualifying user message on or
after that date are returned (Slack's API does not expose a reliable last-activity
timestamp, so messages are fetched to filter). When --with-messages is set, the
latest user-authored message is included in each result object.

Input:
  workspace  Name of the saved workspace (e.g. acme)

Flags:
  --start-from      Only include DMs with activity on or after this date (YYYY-MM-DD)
  --with-messages   Include the latest message for each DM conversation
  --system-events   Include system/bot messages (e.g. "joined Slack" notifications)

Output format (JSON array, stdout):
  [
    {
      "id": "DXXXXXXXX",
      "userId": "UXXXXXXXX",
      "userName": "Jane Doe",
      "isIm": true,
      "lastMessage": {
        "userId": "UXXXXXXXX",
        "text": "message text",
        "timestamp": "2024-01-15 10:30"
      }
    },
    {
      "id": "GXXXXXXXX",
      "name": "mpdm-alice--bob--carol-1",
      "isIm": false
    }
  ]

  userId/userName are present only for 1:1 DMs (isIm: true).
  name is present only for group DMs (isIm: false).
  lastMessage is present only when --with-messages is set.

Examples:
  slack-cli list-dms acme
  slack-cli list-dms acme --start-from 2024-01-01
  slack-cli list-dms acme --with-messages
  slack-cli list-dms acme --with-messages --system-events`,
		Args: cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			sf, err := ParseStartFrom(startFrom)
			if err != nil {
				return err
			}
			cmd := &ListDMsCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
				WithMessages:  withMessages,
				SystemEvents:  systemEvents,
			}
			return cmd.Run(cobraCmd.Context(), args[0], sf)
		},
	}
	c.Flags().StringVar(&startFrom, "start-from", "", "Only include DMs with activity on or after this date (YYYY-MM-DD)")
	c.Flags().BoolVar(&withMessages, "with-messages", false, "Include the latest message for each DM conversation")
	c.Flags().BoolVar(&systemEvents, "system-events", false, "Include system/bot messages (e.g. \"joined Slack\" notifications)")
	return c
}

func newLoadThreadCmd(deps RootDeps) *cobra.Command {
	var startFrom string
	c := &cobra.Command{
		Use:   "load-thread <workspace> <channel-id> <thread-ts>",
		Short: "Load all messages in a Slack thread",
		Long: `Fetches all replies in a Slack thread and writes them as a JSON object to stdout.
User IDs are NOT resolved to display names — use load-context if you need resolved names.

The thread-ts is the raw Slack timestamp of the parent (root) message. Obtain it from:
  - the rawTs field in search-channels output, or
  - a Slack permalink by converting p1700000000123456 → 1700000000.123456

Input:
  workspace   Name of the saved workspace (e.g. acme)
  channel-id  Slack channel ID starting with "C" (e.g. C01234567)
  thread-ts   Thread timestamp (e.g. 1700000000.123456) — the ts of the parent message

Flags:
  --start-from  Only include messages on or after this date (YYYY-MM-DD)

Output format (JSON object, stdout):
  {
    "messages": [
      {
        "userId": "UXXXXXXXX",
        "timestamp": "1700000000.123456",
        "text": "message text",
        "reactions": [{"name": "thumbsup", "count": 2}],
        "files": ["filename.pdf"]
      }
    ],
    "truncated": false
  }

  userId is the raw Slack user ID (not resolved); use get-user to resolve it.
  reactions and files are omitted when empty.
  truncated is true when the thread exceeds the pagination limit.

Examples:
  slack-cli load-thread acme C01234567 1700000000.123456
  slack-cli load-thread acme C01234567 1700000000.123456 --start-from 2024-06-01`,
		Args: cobra.ExactArgs(3),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			sf, err := ParseStartFrom(startFrom)
			if err != nil {
				return err
			}
			cmd := &LoadThreadCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
			}
			return cmd.Run(cobraCmd.Context(), args[0], args[1], args[2], sf)
		},
	}
	c.Flags().StringVar(&startFrom, "start-from", "", "Only messages on or after this date (YYYY-MM-DD)")
	return c
}

func newGetUserCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "get-user <workspace> <user-id>",
		Short: "Resolve a Slack user ID to a display name",
		Long: `Looks up a Slack user by their ID and prints their display name and real name.
Useful for resolving raw user IDs that appear in load-thread output (userId fields).

Input:
  workspace  Name of the saved workspace (e.g. acme)
  user-id    Slack user ID starting with "U" (e.g. U01234567)

Output: plain text — display name and real name on separate lines.

Example:
  slack-cli get-user acme U01234567`,
		Args: cobra.ExactArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cmd := &GetUserCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
			}
			return cmd.Run(cobraCmd.Context(), args[0], args[1])
		},
	}
}

func newLoadContextCmd(deps RootDeps) *cobra.Command {
	var permalink, channelName, searchQuery, startFrom string
	c := &cobra.Command{
		Use:   "load-context <workspace> <channel-id> <thread-ts>",
		Short: "Load a Slack thread with resolved user names (markdown output)",
		Long: `Fetches a Slack thread, resolves all user IDs to display names, and writes
formatted markdown to stdout — ideal for piping into an AI assistant or document.

The thread-ts is the raw Slack timestamp of the parent (root) message. Obtain it from:
  - the rawTs field in search-channels output, or
  - the thread_ts field in search output, or
  - a Slack permalink by converting p1700000000123456 → 1700000000.123456

Input:
  workspace   Name of the saved workspace (e.g. acme)
  channel-id  Slack channel ID starting with "C" (e.g. C01234567)
  thread-ts   Thread timestamp (e.g. 1700000000.123456) — the ts of the parent message

Flags:
  --permalink     Slack permalink URL for the thread (added to output header)
  --channel-name  Human-readable channel name, e.g. "general" (used in header instead of ID)
  --search-query  Label for the markdown heading (e.g. the query that found this thread)
  --start-from    Only include messages on or after this date (YYYY-MM-DD)

Output format (markdown, stdout):
  # Slack Context: "search-query" — #channel-name

  **Source:** #channel-name | **Started:** 2024-01-15 10:30 | **Messages:** 5
  **Permalink:** https://...

  ---

  > **Jane Doe** (2024-01-15 10:30):
  > Message text here
  > _Reactions: :thumbsup: 2_
  > _Attachments: file.pdf_

  ---

  Slack context loaded: **5** message(s) from **#channel-name** (thread: 1700000000.123456).

Examples:
  slack-cli load-context acme C01234567 1700000000.123456
  slack-cli load-context acme C01234567 1700000000.123456 \
    --channel-name general \
    --permalink https://acme.slack.com/archives/C01234567/p1700000000123456 \
    --search-query "deployment failed"`,
		Args: cobra.ExactArgs(3),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			sf, err := ParseStartFrom(startFrom)
			if err != nil {
				return err
			}
			cmd := &LoadContextCommand{
				Store:         deps.Store,
				Output:        deps.Output,
				ClientFactory: deps.ClientFactory,
			}
			return cmd.Run(cobraCmd.Context(), LoadContextArgs{
				Workspace:   args[0],
				ChannelID:   args[1],
				ThreadTS:    args[2],
				Permalink:   permalink,
				ChannelName: channelName,
				SearchQuery: searchQuery,
				StartFrom:   sf,
			})
		},
	}
	c.Flags().StringVar(&permalink, "permalink", "", "Slack permalink URL")
	c.Flags().StringVar(&channelName, "channel-name", "", "Channel display name (e.g. 'general')")
	c.Flags().StringVar(&searchQuery, "search-query", "", "Original search query used as context label")
	c.Flags().StringVar(&startFrom, "start-from", "", "Only messages on or after this date (YYYY-MM-DD)")
	return c
}
