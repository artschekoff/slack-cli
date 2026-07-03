// Command slack-cli manages Slack credentials and exposes Slack operations
// (search, threads, user lookup) as command-line subcommands.
//
// Log in once per machine to store the master passphrase:
//
//	slack-cli login
//	slack-cli logout
//
// After login, every subcommand reads the passphrase from ~/.slack/session.
package main

import (
	"fmt"
	"os"

	"github.com/artschekoff/slack-cli/internal/cli"
	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/session"
)

func main() {
	sessionPath, err := session.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "slack-cli: %v\n", err)
		os.Exit(1)
	}
	sessStore := session.NewStore(sessionPath)

	credStore, err := credentials.NewStore(
		credentials.WithPassphrase(buildPassphraseProvider(sessStore)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slack-cli: initializing credential store: %v\n", err)
		os.Exit(1)
	}

	root := cli.NewRootCmd(cli.DefaultRootDeps(credStore, sessStore))

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "slack-cli: %v\n", err)
		os.Exit(1)
	}
}
