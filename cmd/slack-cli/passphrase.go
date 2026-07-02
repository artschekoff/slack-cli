package main

import (
	"fmt"
	"os"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"golang.org/x/term"
)

const passphraseEnvVar = "SLACK_MCP_PASSPHRASE"

// buildPassphraseProvider returns a PassphraseProvider that tries, in order:
//  1. The SLACK_MCP_PASSPHRASE environment variable (for headless / MCP-server use).
//  2. An interactive terminal prompt on stdin (for CLI use).
//
// If neither source is available, a clear error is returned explaining how to
// set the env var — which is the typical situation when Cursor or another MCP
// host starts the process with stdin wired to the JSON-RPC transport.
func buildPassphraseProvider() credentials.PassphraseProvider {
	return func() (string, error) {
		if pass, err := envPassphraseProvider(); err == nil {
			return pass, nil
		}

		if term.IsTerminal(int(os.Stdin.Fd())) {
			return terminalPassphraseProvider()
		}

		return "", fmt.Errorf(
			"credentials are encrypted but no passphrase is available; "+
				"set the %s environment variable",
			passphraseEnvVar,
		)
	}
}

// envPassphraseProvider reads the passphrase from the SLACK_MCP_PASSPHRASE env var.
func envPassphraseProvider() (string, error) {
	pass := os.Getenv(passphraseEnvVar)
	if pass == "" {
		return "", fmt.Errorf("%s is not set or empty", passphraseEnvVar)
	}
	return pass, nil
}

// terminalPassphraseProvider prompts the user for a master passphrase on the
// terminal, suppressing echo so the passphrase is not visible while being typed.
func terminalPassphraseProvider() (string, error) {
	fmt.Fprint(os.Stderr, "Credentials passphrase: ")

	passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}

	pass := string(passBytes)
	if pass == "" {
		return "", fmt.Errorf("passphrase must not be empty")
	}

	return pass, nil
}
