// Command slack-cli manages Slack credentials and exposes Slack operations
// (search, threads, user lookup) as command-line subcommands.
//
// Run without arguments to see all available commands:
//
//	slack-cli --help
//
// Authenticate a workspace first:
//
//	slack-cli auth [workspace]
//	slack-cli auth-start [workspace]   # non-interactive: shows instructions
//	slack-cli auth-complete <workspace> --token xoxc-... --cookie xoxd-...
package main

import (
	"fmt"
	"os"

	"github.com/artschekoff/slack-cli/internal/cli"
	"github.com/artschekoff/slack-cli/internal/credentials"
)

const version = "1.0.0"

func main() {
	store, err := credentials.NewStore(
		credentials.WithPassphrase(buildPassphraseProvider()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slack-cli: initializing credential store: %v\n", err)
		os.Exit(1)
	}

	root := cli.NewRootCmd(cli.DefaultRootDeps(store))

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "slack-cli: %v\n", err)
		os.Exit(1)
	}
}
