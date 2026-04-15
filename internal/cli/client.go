package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
)

// ClientFactory builds a Slack client from stored credentials.
// Overridable in tests to inject a test-server-aware client.
type ClientFactory func(creds credentials.Creds) *slack.Client

// DefaultClientFactory returns the production Slack client factory.
func DefaultClientFactory() ClientFactory {
	return func(creds credentials.Creds) *slack.Client {
		return slack.NewClient(creds.Token, creds.Cookie)
	}
}

// resolveClient looks up credentials for workspace in store, validates them,
// and returns a configured Slack client using factory.
// Returns a descriptive error (not an unauthorizedError) when credentials are
// missing or incomplete; the caller is responsible for wrapping with a sentinel.
func resolveClient(ctx context.Context, store *credentials.Store, workspace string, factory ClientFactory) (*slack.Client, error) {
	creds, err := store.Get(ctx, workspace)
	if errors.Is(err, credentials.ErrWorkspaceNotFound) {
		return nil, fmt.Errorf("%w: no credentials found for workspace '%s'. Use auth_start to authenticate", ErrCredentialsNotFound, workspace)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials for workspace '%s': %w", workspace, err)
	}
	if creds.Token == "" || creds.Cookie == "" {
		return nil, fmt.Errorf("%w: incomplete credentials for workspace '%s'. Use auth_start to re-authenticate", ErrCredentialsNotFound, workspace)
	}
	return factory(creds), nil
}
