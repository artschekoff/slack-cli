package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestCredsCommand_ValidCredentials(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    store,
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "valid")
	assert.Contains(t, text, "testuser")
	assert.Contains(t, text, "Test Workspace")
}

func TestTestCredsCommand_NoCredentials(t *testing.T) {
	store := newTempStore(t)

	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    store,
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "nonexistent")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "no credentials")
}

func TestTestCredsCommand_ExpiredCredentials(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-stale", Cookie: "xoxd-stale"}))

	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    store,
		Output:   &output,
		Validate: failValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "invalid")
}

func TestTestCredsCommand_NetworkError_Propagated(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	networkErr := errors.New("connection refused")
	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:  store,
		Output: &output,
		Validate: func(_ context.Context, _, _ string) (slack.AuthTestResult, error) {
			return slack.AuthTestResult{}, networkErr
		},
	}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err)
	assert.ErrorIs(t, err, networkErr)
}

func TestTestCredsCommand_InvalidWorkspaceName(t *testing.T) {
	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    newTempStore(t),
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}

func TestTestCredsCommand_RequiresWorkspaceName(t *testing.T) {
	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    newTempStore(t),
		Input:    strings.NewReader("\n"),
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace name is required")
}

func TestTestCredsCommand_PromptsForWorkspace(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    store,
		Input:    strings.NewReader("acme\n"),
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "")
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "Workspace name")
	assert.Contains(t, text, "valid")
}

func TestTestCredsCommand_DoesNotLeakSecrets(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-supersecret", Cookie: "xoxd-supersecret"}))

	var output bytes.Buffer
	cmd := &TestCredsCommand{
		Store:    store,
		Output:   &output,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	text := output.String()
	assert.NotContains(t, text, "xoxc-supersecret", "token must not appear in output")
	assert.NotContains(t, text, "xoxd-supersecret", "cookie must not appear in output")
}

// --- Cobra integration tests ---

func TestRootCmd_HasTestCredsSubcommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCmd(newRootDeps(t, "", &out))

	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "test-creds" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command must have a 'test-creds' subcommand")
}

func TestRootCmd_TestCreds_ValidFromArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)
	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"test-creds", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "valid")
	assert.Contains(t, text, "testuser")
}

func TestRootCmd_TestCreds_NoCredsFromArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"test-creds", "nonexistent"})

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "no credentials")
}

func TestRootCmd_TestCreds_ExpiredFromArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)
	deps.Validate = failValidator
	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-stale", Cookie: "xoxd-stale"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"test-creds", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "invalid")
}

func TestRootCmd_TestCreds_PromptsWhenNoArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "acme\n", &out)
	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"test-creds"})

	err := root.Execute()
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "Workspace name")
	assert.Contains(t, text, "valid")
}
