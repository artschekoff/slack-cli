package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowCredsCommand_EmptyStore(t *testing.T) {
	store := newTempStore(t)
	var output bytes.Buffer

	cmd := &ShowCredsCommand{
		Store:  store,
		Output: &output,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, store.Path(), "output must show the credentials file path")
	assert.Contains(t, text, "No workspaces", "should report empty store")
}

func TestShowCredsCommand_ShowsWorkspaceNames(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-secret-token", Cookie: "xoxd-secret-cookie"}))
	require.NoError(t, store.Save(context.Background(), "globex", credentials.Creds{Token: "xoxc-other-token", Cookie: "xoxd-other-cookie"}))

	var output bytes.Buffer
	cmd := &ShowCredsCommand{
		Store:  store,
		Output: &output,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "acme", "workspace name must appear")
	assert.Contains(t, text, "globex", "workspace name must appear")
}

func TestShowCredsCommand_HidesSecrets(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-super-secret", Cookie: "xoxd-super-cookie"}))

	var output bytes.Buffer
	cmd := &ShowCredsCommand{
		Store:  store,
		Output: &output,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	text := output.String()
	assert.NotContains(t, text, "xoxc-super-secret", "token must not appear in output")
	assert.NotContains(t, text, "xoxd-super-cookie", "cookie must not appear in output")
}

func TestShowCredsCommand_ShowsFilePath(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	store, err := credentials.NewStoreAt(credPath)
	require.NoError(t, err)

	var output bytes.Buffer
	cmd := &ShowCredsCommand{
		Store:  store,
		Output: &output,
	}

	err = cmd.Run(context.Background())
	require.NoError(t, err)

	assert.Contains(t, output.String(), credPath, "credentials file path must be shown")
}

func TestShowCredsCommand_MultipleWorkspaces(t *testing.T) {
	store := newTempStore(t)
	workspaces := []string{"alpha", "beta", "gamma"}
	for _, ws := range workspaces {
		require.NoError(t, store.Save(context.Background(), ws, credentials.Creds{Token: "xoxc-tok", Cookie: "xoxd-cook"}))
	}

	var output bytes.Buffer
	cmd := &ShowCredsCommand{
		Store:  store,
		Output: &output,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	text := output.String()
	for _, ws := range workspaces {
		assert.Contains(t, text, ws)
	}
	assert.NotContains(t, text, "xoxc-tok")
	assert.NotContains(t, text, "xoxd-cook")
}
