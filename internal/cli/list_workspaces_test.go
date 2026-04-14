package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkspacesCommand_Empty(t *testing.T) {
	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: newTempStore(t), Output: &out}

	err := cmd.Run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No workspaces found")
}

func TestListWorkspacesCommand_WithEntries(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "t1", Cookie: "c1"}))
	require.NoError(t, store.Save(context.Background(), "globex", credentials.Creds{Token: "t2", Cookie: "c2"}))

	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background())
	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "acme")
	assert.Contains(t, text, "globex")
	assert.NotContains(t, text, "t1", "tokens must not appear in output")
}

func TestListWorkspacesCommand_StoreError(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, os.WriteFile(store.Path(), []byte("corrupt"), 0o600))

	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background())
	require.Error(t, err)
}
