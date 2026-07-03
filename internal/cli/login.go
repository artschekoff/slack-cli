package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newLoginCmd builds the "login" subcommand.
//
// login prompts for the master passphrase (or reads one line from stdin
// when --stdin is passed), encrypts it, and stores it at ~/.slack/session.
// If workspace credentials already exist, the passphrase is verified by
// attempting to decrypt them; a failing verification deletes the freshly
// written session file to avoid leaving broken state on disk.
func newLoginCmd(deps RootDeps) *cobra.Command {
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store the master passphrase encrypted on disk",
		Long: `Save the master passphrase used to encrypt Slack workspace
credentials. Subsequent commands read the passphrase from the session
file at ~/.slack/session instead of prompting.

Interactive:   slack-cli login
Piped (CI):    echo "$PASS" | slack-cli login --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pass, err := readPassphrase(deps.Input, useStdin)
			if err != nil {
				return err
			}
			if pass == "" {
				return errors.New("passphrase must not be empty")
			}

			if err := deps.SessionStore.Save(pass); err != nil {
				return fmt.Errorf("saving session: %w", err)
			}

			if err := verifyPassphrase(cmd.Context(), deps); err != nil {
				_ = deps.SessionStore.Delete()
				return fmt.Errorf("passphrase does not decrypt stored credentials: %w", err)
			}

			fmt.Fprintln(deps.Output, "logged in — session saved to "+deps.SessionStore.Path())
			return nil
		},
	}
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read passphrase from stdin instead of prompting")
	return cmd
}

// readPassphrase returns the passphrase from stdin (one line, without newline)
// when useStdin is set, otherwise from an interactive terminal prompt.
func readPassphrase(in io.Reader, useStdin bool) (string, error) {
	if useStdin {
		br := bufio.NewReader(in)
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("reading passphrase: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Master passphrase: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	return string(b), nil
}

// verifyPassphrase confirms the freshly saved passphrase actually decrypts
// existing workspace credentials. If no workspaces exist yet, verification
// trivially succeeds.
func verifyPassphrase(ctx context.Context, deps RootDeps) error {
	if ctx == nil {
		ctx = context.Background()
	}
	names, err := deps.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing workspaces: %w", err)
	}
	if len(names) == 0 {
		return nil
	}
	if _, err := deps.Store.Get(ctx, names[0]); err != nil {
		return err
	}
	return nil
}
