package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDeps(t *testing.T, stdin string) (RootDeps, *session.Store, *credentials.Store, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	sessStore := session.NewStore(filepath.Join(dir, "session"))
	credStore, err := credentials.NewStoreAt(
		filepath.Join(dir, "creds.json"),
		credentials.WithPassphrase(func() (string, error) {
			return sessStore.Load()
		}),
	)
	require.NoError(t, err)
	out := &bytes.Buffer{}
	deps := RootDeps{
		Store:        credStore,
		SessionStore: sessStore,
		Input:        strings.NewReader(stdin),
		Output:       out,
	}
	return deps, sessStore, credStore, out
}

func TestLogin_StdinFlag_SavesPassphrase(t *testing.T) {
	deps, sessStore, _, out := newTestDeps(t, "my-pass\n")

	cmd := newLoginCmd(deps)
	cmd.SetArgs([]string{"--stdin"})
	cmd.SetOut(out)
	cmd.SetErr(out)

	require.NoError(t, cmd.Execute())

	got, err := sessStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "my-pass", got)
	assert.Contains(t, out.String(), "logged in")
}

func TestLogin_StdinFlag_EmptyPassphrase_Rejected(t *testing.T) {
	deps, sessStore, _, out := newTestDeps(t, "\n")

	cmd := newLoginCmd(deps)
	cmd.SetArgs([]string{"--stdin"})
	cmd.SetOut(out)
	cmd.SetErr(out)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	_, loadErr := sessStore.Load()
	assert.ErrorIs(t, loadErr, session.ErrNoSession,
		"empty passphrase must not create a session file")
}

func TestLogin_WrongPassphrase_DeletesSession(t *testing.T) {
	deps, sessStore, credStore, out := newTestDeps(t, "wrong-pass\n")

	// Seed credentials store with a workspace encrypted under a different passphrase.
	seedStore, err := credentials.NewStoreAt(
		credStore.Path(),
		credentials.WithPassphrase(func() (string, error) { return "real-pass", nil }),
	)
	require.NoError(t, err)
	require.NoError(t, seedStore.Save(context.Background(), "acme", credentials.Creds{
		Token: "xoxc-x", Cookie: "xoxd-x",
	}))

	cmd := newLoginCmd(deps)
	cmd.SetArgs([]string{"--stdin"})
	cmd.SetOut(out)
	cmd.SetErr(out)

	err = cmd.Execute()
	require.Error(t, err, "wrong passphrase must fail login")

	_, loadErr := sessStore.Load()
	assert.ErrorIs(t, loadErr, session.ErrNoSession,
		"failed verification must roll back the session file")
}

func TestLogin_NoExistingCredentials_Succeeds(t *testing.T) {
	deps, sessStore, _, out := newTestDeps(t, "any-pass\n")

	cmd := newLoginCmd(deps)
	cmd.SetArgs([]string{"--stdin"})
	cmd.SetOut(out)
	cmd.SetErr(out)

	require.NoError(t, cmd.Execute(),
		"login must succeed when no credentials exist yet (nothing to verify against)")

	got, err := sessStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "any-pass", got)
}
