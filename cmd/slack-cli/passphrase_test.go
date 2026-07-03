package main

import (
	"path/filepath"
	"testing"

	"github.com/artschekoff/slack-cli/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionPassphraseProvider_ReturnsStoredPassphrase(t *testing.T) {
	sessStore := session.NewStore(filepath.Join(t.TempDir(), "session"))
	require.NoError(t, sessStore.Save("stored-pass"))

	provider := buildPassphraseProvider(sessStore)
	got, err := provider()
	require.NoError(t, err)
	assert.Equal(t, "stored-pass", got)
}

func TestSessionPassphraseProvider_NoSession_ReturnsGuidance(t *testing.T) {
	sessStore := session.NewStore(filepath.Join(t.TempDir(), "session"))

	provider := buildPassphraseProvider(sessStore)
	_, err := provider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack-cli login",
		"error must tell the user how to fix the missing session")
}
