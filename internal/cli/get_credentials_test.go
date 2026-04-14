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

func TestGetCredentialsCommand_NotFound(t *testing.T) {
	var out bytes.Buffer
	cmd := &GetCredentialsCommand{Store: newTempStore(t), Output: &out}

	err := cmd.Run(context.Background(), "missing")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No credentials found")
}

func TestGetCredentialsCommand_Exists(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.Save(context.Background(), "acme", credentials.Creds{Token: "xoxc-abc", Cookie: "xoxd-xyz"}))

	var out bytes.Buffer
	cmd := &GetCredentialsCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background(), "acme")
	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "present")
	assert.NotContains(t, text, "xoxc-abc", "raw token must not be exposed")
	assert.NotContains(t, text, "xoxd-xyz", "raw cookie must not be exposed")
}

func TestGetCredentialsCommand_StoreError(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, os.WriteFile(store.Path(), []byte("corrupt"), 0o600))

	var out bytes.Buffer
	cmd := &GetCredentialsCommand{Store: store, Output: &out}

	err := cmd.Run(context.Background(), "acme")
	require.Error(t, err)
}
