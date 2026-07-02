package main

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// saveEncryptedCreds creates a temp store with the given passphrase, saves
// credentials, and returns the file path so a second store can open it.
func saveEncryptedCreds(t *testing.T, passphrase string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	store, err := credentials.NewStoreAt(path,
		credentials.WithPassphrase(func() (string, error) { return passphrase, nil }),
	)
	require.NoError(t, err)
	require.NoError(t, store.Save(context.Background(), "jiro", credentials.Creds{
		Token:  "xoxc-test-token",
		Cookie: "xoxd-test-cookie",
	}))
	return path
}

// --- Bug reproduction: passphrase provider that fails ---

func TestEncryptedStore_FailingProvider_CannotGet(t *testing.T) {
	path := saveEncryptedCreds(t, "my-secret")

	failingProvider := func() (string, error) {
		return "", assert.AnError
	}

	store, err := credentials.NewStoreAt(path, credentials.WithPassphrase(failingProvider))
	require.NoError(t, err)

	_, err = store.Get(context.Background(), "jiro")
	require.Error(t, err, "Get must fail when the passphrase provider fails")
	assert.Contains(t, err.Error(), "fetching passphrase")
}

// --- envPassphraseProvider ---

func TestEnvPassphraseProvider_ReturnsEnvVar(t *testing.T) {
	t.Setenv(passphraseEnvVar, "env-secret")

	pass, err := envPassphraseProvider()
	require.NoError(t, err)
	assert.Equal(t, "env-secret", pass)
}

func TestEnvPassphraseProvider_EmptyEnvVar(t *testing.T) {
	t.Setenv(passphraseEnvVar, "")

	_, err := envPassphraseProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), passphraseEnvVar)
}

func TestEnvPassphraseProvider_UnsetEnvVar(t *testing.T) {
	t.Setenv(passphraseEnvVar, "")

	_, err := envPassphraseProvider()
	require.Error(t, err)
}

// --- buildPassphraseProvider ---

func TestBuildPassphraseProvider_PrefersEnvVar(t *testing.T) {
	t.Setenv(passphraseEnvVar, "env-secret")

	provider := buildPassphraseProvider()
	pass, err := provider()
	require.NoError(t, err)
	assert.Equal(t, "env-secret", pass)
}

func TestBuildPassphraseProvider_EnvVarDecryptsStore(t *testing.T) {
	passphrase := "my-master-pass"
	path := saveEncryptedCreds(t, passphrase)

	t.Setenv(passphraseEnvVar, passphrase)

	provider := buildPassphraseProvider()
	store, err := credentials.NewStoreAt(path, credentials.WithPassphrase(provider))
	require.NoError(t, err)

	got, err := store.Get(context.Background(), "jiro")
	require.NoError(t, err, "env-var passphrase must decrypt credentials")
	assert.Equal(t, "xoxc-test-token", got.Token)
	assert.Equal(t, "xoxd-test-cookie", got.Cookie)
}

func TestBuildPassphraseProvider_NoEnvVar_NonTTY_FailsWithGuidance(t *testing.T) {
	t.Setenv(passphraseEnvVar, "")

	provider := buildPassphraseProvider()
	_, err := provider()

	require.Error(t, err)
	assert.Contains(t, err.Error(), passphraseEnvVar,
		"error must mention the env var name so the user knows how to fix it")
}

// --- Caching: provider called once, multiple Gets reuse cached passphrase ---

func TestBuildPassphraseProvider_CachedAcrossMultipleGets(t *testing.T) {
	passphrase := "cache-test-pass"
	path := saveEncryptedCreds(t, passphrase)

	writeStore, err := credentials.NewStoreAt(path,
		credentials.WithPassphrase(func() (string, error) { return passphrase, nil }),
	)
	require.NoError(t, err)
	require.NoError(t, writeStore.Save(context.Background(), "acme", credentials.Creds{
		Token: "xoxc-acme", Cookie: "xoxd-acme",
	}))

	t.Setenv(passphraseEnvVar, passphrase)

	var callCount atomic.Int32
	countingProvider := func() (string, error) {
		callCount.Add(1)
		return envPassphraseProvider()
	}

	readStore, err := credentials.NewStoreAt(path, credentials.WithPassphrase(countingProvider))
	require.NoError(t, err)

	got1, err := readStore.Get(context.Background(), "jiro")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-test-token", got1.Token)

	got2, err := readStore.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "xoxc-acme", got2.Token)

	names, err := readStore.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, names, 2)

	assert.Equal(t, int32(1), callCount.Load(),
		"passphrase provider must be called exactly once; Store caches the result")
}
