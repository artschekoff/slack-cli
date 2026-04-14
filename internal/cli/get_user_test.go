package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserCommand_Success(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"users.info": map[string]any{
			"ok":   true,
			"user": map[string]any{"real_name": "Alice Smith", "name": "alice"},
		},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	var out bytes.Buffer
	cmd := &GetUserCommand{Store: store, Output: &out, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "U001")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Alice Smith")
	assert.Contains(t, out.String(), "U001")
}

func TestGetUserCommand_Unauthorized(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"users.info": map[string]any{"ok": false, "error": "token_revoked"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &GetUserCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "U001")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestGetUserCommand_APIError(t *testing.T) {
	srv := newSlackTestServer(t, map[string]any{
		"users.info": map[string]any{"ok": false, "error": "user_not_found"},
	})

	store := storeWithCredsForCLI(t, "acme", "xoxc-test", "xoxd-test")
	cmd := &GetUserCommand{Store: store, Output: &bytes.Buffer{}, ClientFactory: newTestClientFactory(t, srv)}

	err := cmd.Run(context.Background(), "acme", "U001")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlackGetUser)
}

func TestGetUserCommand_WorkspaceNotFound(t *testing.T) {
	cmd := &GetUserCommand{Store: newTempStore(t), Output: &bytes.Buffer{}, ClientFactory: DefaultClientFactory()}

	err := cmd.Run(context.Background(), "missing", "U001")
	require.Error(t, err)
}
