package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/artschekoff/slack-cli/internal/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthCompleteCommand_Success(t *testing.T) {
	store := newTempStore(t)
	var out bytes.Buffer

	cmd := &AuthCompleteCommand{
		Store:    store,
		Output:   &out,
		Validate: successValidator,
	}

	err := cmd.Run(context.Background(), "acme", "xoxc-token", "xoxd-cookie")
	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Authentication successful")
	assert.Contains(t, text, "testuser")
	assert.Contains(t, text, "Test Workspace")

	saved, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-token", saved.Token)
	assert.Equal(t, "xoxd-cookie", saved.Cookie)
}

func TestAuthCompleteCommand_InvalidTokenPrefix(t *testing.T) {
	cmd := &AuthCompleteCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, Validate: successValidator}

	err := cmd.Run(context.Background(), "acme", "bad-token", "xoxd-cookie")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xoxc-")
}

func TestAuthCompleteCommand_InvalidCookiePrefix(t *testing.T) {
	cmd := &AuthCompleteCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, Validate: successValidator}

	err := cmd.Run(context.Background(), "acme", "xoxc-tok", "bad-cookie")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xoxd-")
}

func TestAuthCompleteCommand_Unauthorized(t *testing.T) {
	cmd := &AuthCompleteCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, Validate: failValidator}

	err := cmd.Run(context.Background(), "acme", "xoxc-tok", "xoxd-cook")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestAuthCompleteCommand_NetworkError(t *testing.T) {
	netErr := func(_ context.Context, _, _ string) (slack.AuthTestResult, error) {
		return slack.AuthTestResult{}, slack.ErrRateLimited
	}
	cmd := &AuthCompleteCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, Validate: netErr}

	err := cmd.Run(context.Background(), "acme", "xoxc-tok", "xoxd-cook")
	require.Error(t, err)
}

func TestAuthCompleteCommand_SaveError(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, os.WriteFile(store.Path(), []byte("corrupt"), 0o600))

	cmd := &AuthCompleteCommand{Store: store, Output: &bytes.Buffer{}, Validate: successValidator}

	err := cmd.Run(context.Background(), "acme", "xoxc-tok", "xoxd-cook")
	require.Error(t, err)
}

func TestAuthCompleteCommand_InvalidWorkspace(t *testing.T) {
	cmd := &AuthCompleteCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, Validate: successValidator}

	err := cmd.Run(context.Background(), "../etc", "xoxc-tok", "xoxd-cook")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace name")
}
