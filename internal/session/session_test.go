package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/artschekoff/slack-cli/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)

	require.NoError(t, s.Save("my-secret-pass"))

	got, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, "my-secret-pass", got)
}

func TestStore_Load_MissingFile_ReturnsErrNoSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)

	_, err := s.Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, session.ErrNoSession))
}

func TestStore_Save_EmptyPassphrase_Rejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)

	err := s.Save("")
	require.Error(t, err)
}

func TestStore_Save_FileHasMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)

	require.NoError(t, s.Save("x"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStore_Load_TamperedCiphertext_Errors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)
	require.NoError(t, s.Save("x"))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw[len(raw)-1] ^= 0xFF
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	_, err = s.Load()
	require.Error(t, err)
	assert.False(t, errors.Is(err, session.ErrNoSession),
		"tampered file must not be reported as 'no session'")
}

func TestStore_Delete_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	s := session.NewStore(path)

	require.NoError(t, s.Delete(), "delete on missing file must not error")

	require.NoError(t, s.Save("x"))
	require.NoError(t, s.Delete())

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Save_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "session")
	s := session.NewStore(path)

	require.NoError(t, s.Save("x"))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}
