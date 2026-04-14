package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveCredsCommand_RemovesExistingWorkspace(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader(""),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	assert.Contains(t, output.String(), "acme", "output must mention the removed workspace")

	_, getErr := store.Get(context.Background(), "acme")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound, "workspace must be removed from store")
}

func TestRemoveCredsCommand_WorkspaceNotFound(t *testing.T) {
	store := newTempStore(t)

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader(""),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestRemoveCredsCommand_PromptsForWorkspaceWhenEmpty(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "globex", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader("globex\n"),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "")
	require.NoError(t, err)

	_, getErr := store.Get(context.Background(), "globex")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound, "workspace must be removed after interactive prompt")
}

func TestRemoveCredsCommand_EmptyWorkspaceInputReturnsError(t *testing.T) {
	store := newTempStore(t)

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader("\n"),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace name is required")
}

func TestRemoveCredsCommand_EOFInputReturnsError(t *testing.T) {
	store := newTempStore(t)

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader(""),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace name is required")
}

func TestRemoveCredsCommand_OnlyRemovesTargetWorkspace(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-a", Cookie: "xoxd-a"}))
	require.NoError(t, store.Save(context.Background(), "globex", credentials.Creds{Token: "xoxc-g", Cookie: "xoxd-g"}))

	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  store,
		Input:  strings.NewReader(""),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)

	_, getErr := store.Get(context.Background(), "acme")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound, "acme must be removed")

	creds, getErr := store.Get(context.Background(), "globex")
	require.NoError(t, getErr, "globex must still be present")
	assert.Equal(t, "xoxc-g", creds.Token)
}

func TestRemoveCredsCommand_InvalidWorkspaceName_Arg(t *testing.T) {
	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  newTempStore(t),
		Input:  strings.NewReader(""),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}

func TestRemoveCredsCommand_InvalidWorkspaceName_Prompted(t *testing.T) {
	var output bytes.Buffer
	cmd := &RemoveCredsCommand{
		Store:  newTempStore(t),
		Input:  strings.NewReader("FOO BAR\n"),
		Output: &output,
	}

	err := cmd.Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}

// --- Cobra integration tests ---

func TestRootCmd_HasRemoveCredsSubcommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCmd(newRootDeps(t, "", &out))

	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "remove-creds" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command must have a 'remove-creds' subcommand")
}

func TestRootCmd_RemoveCreds_RemovesWorkspaceFromArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)
	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"remove-creds", "acme"})

	err := root.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "acme")

	_, getErr := deps.Store.Get(context.Background(), "acme")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound)
}

func TestRootCmd_RemoveCreds_PromptsWhenNoArg(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "acme\n", &out)
	require.NoError(t, deps.Store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))

	root := NewRootCmd(deps)
	root.SetArgs([]string{"remove-creds"})

	err := root.Execute()
	require.NoError(t, err)

	_, getErr := deps.Store.Get(context.Background(), "acme")
	assert.ErrorIs(t, getErr, credentials.ErrWorkspaceNotFound)
}

func TestRootCmd_RemoveCreds_WorkspaceNotFoundReturnsError(t *testing.T) {
	var out bytes.Buffer
	deps := newRootDeps(t, "", &out)

	root := NewRootCmd(deps)
	root.SetArgs([]string{"remove-creds", "doesnotexist"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "doesnotexist")
}
