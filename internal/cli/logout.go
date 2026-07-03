package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newLogoutCmd builds the "logout" subcommand. It deletes the encrypted
// session file; missing file is not an error (idempotent).
func newLogoutCmd(deps RootDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored master passphrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deps.SessionStore.Delete(); err != nil {
				return fmt.Errorf("deleting session: %w", err)
			}
			fmt.Fprintln(deps.Output, "logged out — session removed")
			return nil
		},
	}
}
