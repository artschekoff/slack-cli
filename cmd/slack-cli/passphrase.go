package main

import (
	"errors"
	"fmt"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/session"
)

// buildPassphraseProvider returns a PassphraseProvider that reads the master
// passphrase from the encrypted session file. When the session is missing,
// the returned error tells the user how to create one.
func buildPassphraseProvider(sessStore *session.Store) credentials.PassphraseProvider {
	return func() (string, error) {
		pass, err := sessStore.Load()
		if err != nil {
			if errors.Is(err, session.ErrNoSession) {
				return "", fmt.Errorf("no active session; run: slack-cli login")
			}
			return "", fmt.Errorf("loading session: %w", err)
		}
		return pass, nil
	}
}
