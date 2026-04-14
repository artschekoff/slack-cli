package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTempStore(t *testing.T) *credentials.Store {
	t.Helper()
	s, err := credentials.NewStoreAt(filepath.Join(t.TempDir(), "creds.json"))
	require.NoError(t, err)
	return s
}

func successValidator(_ context.Context, _, _ string) (slack.AuthTestResult, error) {
	return slack.AuthTestResult{
		UserID: "U123",
		User:   "testuser",
		TeamID: "T456",
		Team:   "Test Workspace",
		URL:    "https://test.slack.com/",
	}, nil
}

func failValidator(_ context.Context, _, _ string) (slack.AuthTestResult, error) {
	return slack.AuthTestResult{}, slack.ErrUnauthorized
}

func nopBrowser(_ string) error { return nil }

func TestAuthCommandRun_SuccessWithWorkspaceArg(t *testing.T) {
	store := newTempStore(t)
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-token123\nxoxd-cookie456\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "testuser")
	assert.Contains(t, text, "Test Workspace")

	creds, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-token123", creds.Token)
	assert.Equal(t, "xoxd-cookie456", creds.Cookie)
}

func TestAuthCommandRun_PromptsForWorkspace(t *testing.T) {
	store := newTempStore(t)
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("myteam\nxoxc-tok\nxoxd-cook\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "Workspace name")

	creds, err := store.Get(context.Background(), "myteam")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-tok", creds.Token)
	assert.Equal(t, "xoxd-cook", creds.Cookie)
}

func TestAuthCommandRun_EmptyWorkspaceInput(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace name is required")
}

func TestAuthCommandRun_InvalidTokenPrefix(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("bad-token\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xoxc-")
}

func TestAuthCommandRun_InvalidCookiePrefix(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-valid\nbad-cookie\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xoxd-")
}

func TestAuthCommandRun_ValidationFails(t *testing.T) {
	store := newTempStore(t)
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-tok\nxoxd-cook\n"),
		Output:      &output,
		Validate:    failValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err)
	assert.ErrorIs(t, err, slack.ErrUnauthorized)

	_, getErr := store.Get(context.Background(), "acme")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound, "credentials must not be saved on validation failure")
}

func TestAuthCommandRun_BrowserFailContinues(t *testing.T) {
	store := newTempStore(t)
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: func(_ string) error { return errors.New("no browser") },
		Input:       strings.NewReader("xoxc-tok\nxoxd-cook\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "could not open browser")

	creds, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-tok", creds.Token)
}

func TestAuthCommandRun_StripsQuotesFromToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		cookie    string
		wantToken string
		wantCook  string
	}{
		{
			name:      "single-quoted token and cookie",
			token:     "'xoxc-token123'",
			cookie:    "'xoxd-cookie456'",
			wantToken: "xoxc-token123",
			wantCook:  "xoxd-cookie456",
		},
		{
			name:      "double-quoted token and cookie",
			token:     `"xoxc-token123"`,
			cookie:    `"xoxd-cookie456"`,
			wantToken: "xoxc-token123",
			wantCook:  "xoxd-cookie456",
		},
		{
			name:      "unquoted token and cookie (no change)",
			token:     "xoxc-token123",
			cookie:    "xoxd-cookie456",
			wantToken: "xoxc-token123",
			wantCook:  "xoxd-cookie456",
		},
		{
			name:      "mixed quotes: single token, double cookie",
			token:     "'xoxc-mixed'",
			cookie:    `"xoxd-mixed"`,
			wantToken: "xoxc-mixed",
			wantCook:  "xoxd-mixed",
		},
		{
			name:      "whitespace around quotes",
			token:     "  'xoxc-spaced'  ",
			cookie:    `  "xoxd-spaced"  `,
			wantToken: "xoxc-spaced",
			wantCook:  "xoxd-spaced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTempStore(t)
			var output bytes.Buffer

			cmd := &AuthCommand{
				Store:       store,
				OpenBrowser: nopBrowser,
				Input:       strings.NewReader(tt.token + "\n" + tt.cookie + "\n"),
				Output:      &output,
				Validate:    successValidator,
			}

			err := cmd.Run(context.Background(), "acme")
			require.NoError(t, err)

			creds, err := store.Get(context.Background(), "acme")
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, creds.Token)
			assert.Equal(t, tt.wantCook, creds.Cookie)
		})
	}
}

func TestAuthCommandRun_PrintsInstructions(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-tok\nxoxd-cook\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "DevTools")
	assert.Contains(t, text, "localStorage")
	assert.Contains(t, text, "xoxc-")
	assert.Contains(t, text, "xoxd-")
}

func TestAuthCommandRun_ExistingValidCredentials(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-existing", Cookie: "xoxd-existing"}))

	var output bytes.Buffer
	browserOpened := false

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: func(_ string) error { browserOpened = true; return nil },
		Input:       strings.NewReader(""), // no input needed — credentials reused
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	assert.False(t, browserOpened, "browser must not open when existing credentials are valid")

	text := output.String()
	assert.Contains(t, text, "Credentials already valid")

	creds, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-existing", creds.Token, "credentials must not be overwritten")
}

func TestAuthCommandRun_ExistingInvalidCredentials_Reauth(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-stale", Cookie: "xoxd-stale"}))

	callCount := 0
	validator := func(ctx context.Context, token, cookie string) (slack.AuthTestResult, error) {
		callCount++
		if token == "xoxc-stale" {
			return slack.AuthTestResult{}, slack.ErrUnauthorized
		}
		return slack.AuthTestResult{UserID: "U123", User: "newuser", TeamID: "T456", Team: "Acme"}, nil
	}

	var output bytes.Buffer
	browserOpened := false

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: func(_ string) error { browserOpened = true; return nil },
		Input:       strings.NewReader("xoxc-new\nxoxd-new\n"),
		Output:      &output,
		Validate:    validator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	assert.True(t, browserOpened, "browser must open to re-authenticate")

	text := output.String()
	assert.Contains(t, text, "expired", "should inform user credentials are expired")

	creds, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-new", creds.Token, "new credentials must be saved")
	assert.Equal(t, "xoxd-new", creds.Cookie)

	assert.Equal(t, 2, callCount, "validator must be called twice: once for stale, once for new creds")
}

func TestAuthCommandRun_NetworkErrorDuringValidation_Propagated(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-ok", Cookie: "xoxd-ok"}))

	networkErr := errors.New("connection refused")
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader(""),
		Output:      &output,
		Validate: func(_ context.Context, _, _ string) (slack.AuthTestResult, error) {
			return slack.AuthTestResult{}, networkErr
		},
	}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err, "network errors must propagate, not be swallowed as 'expired'")
	assert.ErrorIs(t, err, networkErr)
	assert.NotContains(t, output.String(), "expired",
		"should not mislead user by claiming credentials expired on a network failure")
}

func TestAuthCommandRun_UnauthorizedError_TriggersReauth(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-stale", Cookie: "xoxd-stale"}))

	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       store,
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-fresh\nxoxd-fresh\n"),
		Output:      &output,
		Validate: func(_ context.Context, token, _ string) (slack.AuthTestResult, error) {
			if token == "xoxc-stale" {
				return slack.AuthTestResult{}, slack.ErrUnauthorized
			}
			return slack.AuthTestResult{UserID: "U1", User: "u", TeamID: "T1", Team: "T"}, nil
		},
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err, "ErrUnauthorized should trigger re-auth, not fail")
	assert.Contains(t, output.String(), "expired")

	creds, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-fresh", creds.Token)
}

func TestAuthCommandRun_InstructionsUseBoldFormatting(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("xoxc-tok\nxoxd-cook\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	bold := "\033[1m"
	reset := "\033[0m"

	assert.Contains(t, text, bold, "instructions should contain ANSI bold sequences")
	assert.Contains(t, text, reset, "instructions should reset ANSI formatting")
	assert.Contains(t, text, bold+"d"+reset, "cookie name 'd' should be bold")
	assert.Contains(t, text, bold+"JSON.parse", "JS command should start with bold")
}

func TestAuthCommandRun_InvalidWorkspaceName_Arg(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader(""),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}

func TestAuthCommandRun_InvalidWorkspaceName_Prompted(t *testing.T) {
	var output bytes.Buffer

	cmd := &AuthCommand{
		Store:       newTempStore(t),
		OpenBrowser: nopBrowser,
		Input:       strings.NewReader("FOO/BAR\n"),
		Output:      &output,
		Validate:    successValidator,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}
