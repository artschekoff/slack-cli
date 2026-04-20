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
  slack-cli auth-complete <workspace>           save credentials non-interactively
  slack-cli list-workspaces                     list saved workspaces
  slack-cli get-credentials <workspace>         check credential status
  slack-cli show-creds                          show credentials file path
  slack-cli test-creds [workspace]              validate stored credentials
  slack-cli remove-creds [workspace]            delete stored credentials

Slack operations:
  slack-cli search <workspace> <query>                  search messages
  slack-cli search-channels <workspace> <pattern>       list channels whose names contain pattern (JSON)
  slack-cli list-dms <workspace>                        list direct message conversations (JSON)
  slack-cli load-thread <workspace> <ch> <ts>           load a thread
  slack-cli load-context <workspace> <ch> <ts>          load thread as markdown for AI
  slack-cli get-user <workspace> <user-id>              resolve user ID to display name

Use "slack-cli <command> --help" for flags and examples for each command.`,
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
stored and still valid, they are reused without re-authenticating.`,
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
to ~/.slack/workspace_credentials.json.`,
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
~/.slack/workspace_credentials.json. If no workspace is given, you will be prompted.`,
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
workspace by calling auth.test. Reports whether the credentials are present
and valid without modifying the store. If no workspace is given, you will be prompted.`,
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
(xoxd-...) from the browser DevTools.

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
then saves them for the workspace. Unlike "auth", this command is fully
non-interactive and suitable for scripting or CI.

Flags:
  --token   Slack user token (required, must start with "xoxc-")
  --cookie  Slack browser cookie "d" value (required, must start with "xoxd-")

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
		Long: `Searches Slack messages using the given query and prints matching results
with channel, author, timestamp, permalink, and message text.

Flags:
  --count       Maximum number of results to return (default 20, max 100)
  --start-from  Only include messages on or after this date (YYYY-MM-DD)

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
		Long: `Lists all accessible Slack channels whose names contain the given substring
and writes a JSON array of {id, name, messages} objects to stdout.

Matching is case-insensitive and treats hyphens as spaces, so "epic 970"
and "970" both match a channel named "epic-970".

By default system notifications (joined/left, topic changes, etc.) and bot/app
messages are filtered out. Use the flags below to include them.

Flags:
  --system-events  Include system notification messages (channel_join, channel_leave, etc.)
  --bot-messages   Include bot and app integration messages

Examples:
  slack-cli search-channels acme 970
  slack-cli search-channels acme "epic 970"
  slack-cli search-channels acme deploy --system-events
  slack-cli search-channels acme deploy --bot-messages
  slack-cli search-channels acme deploy --system-events --bot-messages`,
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
	var withMessages bool
	c := &cobra.Command{
		Use:   "list-dms <workspace>",
		Short: "List direct message conversations with resolved user names (JSON output)",
		Long: `Lists all accessible Slack direct message conversations (1:1 and group DMs)
and writes a JSON array of {id, userId, userName, name, isIm} objects to stdout.

For 1:1 DMs, user IDs are resolved to display names. Group DMs (mpim) include
the auto-generated conversation name.

Flags:
  --start-from     Only include DMs created on or after this date (YYYY-MM-DD)
  --with-messages  Include the latest message for each DM conversation

Examples:
  slack-cli list-dms acme
  slack-cli list-dms acme --start-from 2024-01-01
  slack-cli list-dms acme --with-messages`,
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
			}
			return cmd.Run(cobraCmd.Context(), args[0], sf)
		},
	}
	c.Flags().StringVar(&startFrom, "start-from", "", "Only include DMs created on or after this date (YYYY-MM-DD)")
	c.Flags().BoolVar(&withMessages, "with-messages", false, "Include the latest message for each DM conversation")
	return c
}

func newLoadThreadCmd(deps RootDeps) *cobra.Command {
	var startFrom string
	c := &cobra.Command{
		Use:   "load-thread <workspace> <channel-id> <thread-ts>",
		Short: "Load all messages in a Slack thread",
		Long: `Fetches all replies in a Slack thread and prints them with author,
timestamp, reactions, and file attachments.

Arguments:
  workspace   Name of the saved workspace (e.g. acme)
  channel-id  Slack channel ID (e.g. C01234567)
  thread-ts   Thread timestamp (e.g. 1700000000.123456) — the ts of the parent message

Flags:
  --start-from  Only include messages on or after this date (YYYY-MM-DD)

Example:
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
Useful for resolving the raw user IDs that appear in message payloads.

Arguments:
  workspace  Name of the saved workspace (e.g. acme)
  user-id    Slack user ID starting with "U" (e.g. U01234567)

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
		Long: `Fetches a Slack thread, resolves all user IDs to display names, and
formats the result as markdown — ideal for pasting into AI assistants or docs.

Arguments:
  workspace   Name of the saved workspace (e.g. acme)
  channel-id  Slack channel ID (e.g. C01234567)
  thread-ts   Thread timestamp (e.g. 1700000000.123456) — the ts of the parent message

Flags:
  --permalink     Slack permalink URL for the thread (added to output header)
  --channel-name  Channel display name, e.g. "general" (added to output header)
  --search-query  Original search query used as a context label in the output
  --start-from    Only include messages on or after this date (YYYY-MM-DD)

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
