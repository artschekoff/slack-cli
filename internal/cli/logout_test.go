package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/artschekoff/slack-cli/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogout_RemovesSessionFile(t *testing.T) {
	sessStore := session.NewStore(filepath.Join(t.TempDir(), "session"))
	require.NoError(t, sessStore.Save("x"))

	out := &bytes.Buffer{}
	cmd := newLogoutCmd(RootDeps{SessionStore: sessStore, Output: out})
	cmd.SetOut(out)
	cmd.SetErr(out)

	require.NoError(t, cmd.Execute())

	_, err := sessStore.Load()
	assert.True(t, errors.Is(err, session.ErrNoSession))
	assert.Contains(t, out.String(), "logged out")
}

func TestLogout_NoSession_Idempotent(t *testing.T) {
	sessStore := session.NewStore(filepath.Join(t.TempDir(), "session"))

	out := &bytes.Buffer{}
	cmd := newLogoutCmd(RootDeps{SessionStore: sessStore, Output: out})
	cmd.SetOut(out)
	cmd.SetErr(out)

	require.NoError(t, cmd.Execute(), "logout with no session must not error")
}
