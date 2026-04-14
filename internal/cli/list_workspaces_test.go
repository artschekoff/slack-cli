package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
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
	assert.Equal(t, "", out.String(), "empty store should produce no output")
}

func TestListWorkspacesCommand_WithEntries(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "t1", Cookie: "c1"}))
	require.NoError(t, store.Save(context.Background(), "globex", credentials.Creds{Token: "t2", Cookie: "c2"}))

	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background())
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	assert.ElementsMatch(t, []string{"acme", "globex"}, lines, "output must be workspace names only, one per line")
	assert.NotContains(t, out.String(), "t1", "tokens must not appear in output")
	assert.NotContains(t, out.String(), "Saved", "output must contain no header text")
	assert.NotContains(t, out.String(), "-", "output must contain no bullet points")
}

func TestListWorkspacesCommand_SingleEntry(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "myteam", credentials.Creds{Token: "t1", Cookie: "c1"}))

	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "myteam\n", out.String(), "single workspace: just the name followed by newline")
}

func TestListWorkspacesCommand_StoreError(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, os.WriteFile(store.Path(), []byte("corrupt"), 0o600))

	var out bytes.Buffer
	cmd := &ListWorkspacesCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background())
	require.Error(t, err)
}
