package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/session"
	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRootDeps constructs injectable deps wired to a temp store and a
// success validator. Callers can override individual fields.
func newRootDeps(t *testing.T, input string, out *bytes.Buffer) RootDeps {
	t.Helper()
	return RootDeps{
		Store:        newTempStore(t),
		SessionStore: session.NewStore(filepath.Join(t.TempDir(), "session")),
		OpenBrowser:  nopBrowser,
		Input:        strings.NewReader(input),
		Output:       out,
		Validate:     successValidator,
	}
}

// --- command registration ---

func TestRootCmd_HasAuthSubcommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCmd(newRootDeps(t, "", &out))

	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "auth" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command must have an 'auth' subcommand")
}

func TestRootCmd_HasShowCredsSubcommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCmd(newRootDeps(t, "", &out))

	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "show-creds" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command must have a 'show-creds' subcommand")
}

// --- auth subcommand ---

func TestRootCmd_Auth_WorkspaceFromPositionalArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "xoxc-token\nxoxd-cookie\n", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "testuser")
	assert.Contains(t, text, "Test Workspace")

	creds, err := deps.Store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-token", creds.Token)
	assert.Equal(t, "xoxd-cookie", creds.Cookie)
}

func TestRootCmd_Auth_PromptsWhenNoArg(t *testing.T) {
	var out bytes.Buffer
	// workspace entered interactively, then token+cookie
	deps := newRootDeps(t, "myteam\nxoxc-tok\nxoxd-cook\n", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "Workspace name")

	creds, err := deps.Store.Get(context.Background(), "myteam")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-tok", creds.Token)
}

func TestRootCmd_Auth_ExistingValidCredentials_SkipsBrowser(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{
		Token:  "xoxc-existing",
		Cookie: "xoxd-existing",
	}))

	browserOpened := false
	deps.OpenBrowser = func(_ string) error { browserOpened = true; return nil }

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	assert.False(t, browserOpened, "browser must not open for valid stored creds")
	assert.Contains(t, out.String(), "Credentials already valid")
}

func TestRootCmd_Auth_ExpiredCredentials_Reauthenticates(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "xoxc-new\nxoxd-new\n", &out)

	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{
		Token:  "xoxc-stale",
		Cookie: "xoxd-stale",
	}))

	deps.Validate = func(_ context.Context, token, _ string) (slack.AuthTestResult, error) {
		if token == "xoxc-stale" {
			return slack.AuthTestResult{}, slack.ErrUnauthorized
		}
		return slack.AuthTestResult{UserID: "U1", User: "newuser", TeamID: "T1", Team: "Acme"}, nil
	}

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "expired")

	creds, err := deps.Store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-new", creds.Token)
}

func TestRootCmd_Auth_InvalidToken_ReturnsError(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "bad-token\n", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth", "acme"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xoxc-")
}

// --- show-creds subcommand ---

func TestRootCmd_ShowCreds_EmptyStore(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"show-creds"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, deps.Store.Path())
	assert.Contains(t, text, "No workspaces")
}

func TestRootCmd_ShowCreds_ShowsWorkspaceNames(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"}))
	require.NoError(t, deps.Store.Save(context.Background(), "globex", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"show-creds"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "acme")
	assert.Contains(t, text, "globex")
	assert.NotContains(t, text, "xoxc-t", "tokens must not appear in output")
	assert.NotContains(t, text, "xoxd-c", "cookies must not appear in output")
}

func TestRootCmd_ShowCreds_ShowsFilePath(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"show-creds"})

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), deps.Store.Path())
}

// --- signal context propagation ---

// TestRootCmd_Auth_UsesSignalContext verifies that the auth subcommand wires a
// cancellable context (via signal.NotifyContext) rather than context.Background().
// context.Background().Done() returns nil; a signal-aware context returns a real
// channel, so Ctrl-C during ValidateToken is propagated immediately.
func TestRootCmd_Auth_UsesSignalContext(t *testing.T) {
	var out bytes.Buffer
	var capturedCtx context.Context

	deps := newRootDeps(t, "xoxc-tok\nxoxd-cook\n", &out)
	deps.Validate = func(ctx context.Context, token, cookie string) (slack.AuthTestResult, error) {
		capturedCtx = ctx
		return successValidator(ctx, token, cookie)
	}

	root := NewRootCmd(deps)
	root.SetArgs([]string{"auth", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	require.NotNil(t, capturedCtx, "validator must be called with a context")
	assert.NotNil(t, capturedCtx.Done(),
		"auth command must pass a cancellable context (not context.Background()) so that "+
			"Ctrl-C during ValidateToken is propagated and the user is not stuck waiting for the HTTP timeout")
}
