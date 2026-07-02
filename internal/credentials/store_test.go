package credentials_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTempStore(t *testing.T) *credentials.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := credentials.NewStoreAt(filepath.Join(dir, "creds.json"))
	require.NoError(t, err)
	return store
}

func TestNewStoreAt_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "creds.json")

	store, err := credentials.NewStoreAt(path)
	require.NoError(t, err)
	assert.NotNil(t, store)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestNewStoreAt_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	// Create once
	store1, err := credentials.NewStoreAt(path)
	require.NoError(t, err)
	require.NoError(t, store1.Save(context.Background(), "ws1", credentials.Creds{Token: "tok", Cookie: "ck"}))

	// Open again — must not wipe existing content
	store2, err := credentials.NewStoreAt(path)
	require.NoError(t, err)

	got, err := store2.Get(context.Background(), "ws1")
	require.NoError(t, err)
	assert.Equal(t, "tok", got.Token)
}

func TestStore_ListEmpty(t *testing.T) {
	store := newTempStore(t)

	names, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestStore_SaveAndGet(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		creds     credentials.Creds
	}{
		{
			name:      "simple workspace",
			workspace: "jiro",
			creds:     credentials.Creds{Token: "xoxc-abc", Cookie: "xoxd-xyz"},
		},
		{
			name:      "workspace with hyphen",
			workspace: "acme-corp",
			creds:     credentials.Creds{Token: "xoxc-123", Cookie: "xoxd-456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTempStore(t)

			require.NoError(t, store.Save(context.Background(), tt.workspace, tt.creds))

			got, err := store.Get(context.Background(), tt.workspace)
			require.NoError(t, err)
			assert.Equal(t, tt.creds.Token, got.Token)
			assert.Equal(t, tt.creds.Cookie, got.Cookie)
		})
	}
}

func TestStore_SaveOverwrites(t *testing.T) {
	store := newTempStore(t)

	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "old-tok", Cookie: "old-ck"}))
	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "new-tok", Cookie: "new-ck"}))

	got, err := store.Get(context.Background(), "ws")
	require.NoError(t, err)
	assert.Equal(t, "new-tok", got.Token)
	assert.Equal(t, "new-ck", got.Cookie)
}

func TestStore_GetNotFound(t *testing.T) {
	store := newTempStore(t)

	_, err := store.Get(context.Background(), "nonexistent")
	require.ErrorIs(t, err, credentials.ErrWorkspaceNotFound)
}

func TestStore_List(t *testing.T) {
	store := newTempStore(t)

	require.NoError(t, store.Save(context.Background(), "beta", credentials.Creds{Token: "t1", Cookie: "c1"}))
	require.NoError(t, store.Save(context.Background(), "alpha", credentials.Creds{Token: "t2", Cookie: "c2"}))
	require.NoError(t, store.Save(context.Background(), "gamma", credentials.Creds{Token: "t3", Cookie: "c3"}))

	names, err := store.List(context.Background())
	require.NoError(t, err)
	sort.Strings(names)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestStore_Delete(t *testing.T) {
	store := newTempStore(t)

	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "tok", Cookie: "ck"}))
	require.NoError(t, store.Delete(context.Background(), "ws"))

	_, err := store.Get(context.Background(), "ws")
	require.ErrorIs(t, err, credentials.ErrWorkspaceNotFound)
}

func TestStore_DeleteNotFound(t *testing.T) {
	store := newTempStore(t)

	err := store.Delete(context.Background(), "nonexistent")
	require.ErrorIs(t, err, credentials.ErrWorkspaceNotFound)
}

func TestStore_ListAfterDelete(t *testing.T) {
	store := newTempStore(t)

	require.NoError(t, store.Save(context.Background(), "ws1", credentials.Creds{Token: "t1", Cookie: "c1"}))
	require.NoError(t, store.Save(context.Background(), "ws2", credentials.Creds{Token: "t2", Cookie: "c2"}))
	require.NoError(t, store.Delete(context.Background(), "ws1"))

	names, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"ws2"}, names)
}

func TestStore_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	store, err := credentials.NewStoreAt(path)
	require.NoError(t, err)
	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "tok", Cookie: "ck"}))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "credentials file should be owner-only readable")
}

func TestStore_ConcurrentReadWrite(t *testing.T) {
	store := newTempStore(t)

	// Save an initial entry
	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "t", Cookie: "c"}))

	// Run concurrent reads and writes — race detector will catch issues
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = store.List(context.Background())
			_, _ = store.Get(context.Background(), "ws")
		}()
	}
	for i := 0; i < 5; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			_ = store.Save(context.Background(), "ws", credentials.Creds{Token: "t", Cookie: "c"})
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStore_MultipleWorkspaces_Independent(t *testing.T) {
	store := newTempStore(t)

	require.NoError(t, store.Save(context.Background(), "ws1", credentials.Creds{Token: "tok1", Cookie: "ck1"}))
	require.NoError(t, store.Save(context.Background(), "ws2", credentials.Creds{Token: "tok2", Cookie: "ck2"}))

	// Delete ws1 — ws2 must remain intact
	require.NoError(t, store.Delete(context.Background(), "ws1"))

	c2, err := store.Get(context.Background(), "ws2")
	require.NoError(t, err)
	assert.Equal(t, "tok2", c2.Token)

	_, err = store.Get(context.Background(), "ws1")
	require.ErrorIs(t, err, credentials.ErrWorkspaceNotFound)
}

// ── Encrypted store tests ──────────────────────────────────────────────────

func init() {
	credentials.SetTestCryptoParams()
}

func newTempEncryptedStore(t *testing.T, passphrase string) *credentials.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := credentials.NewStoreAt(
		filepath.Join(dir, "creds.json"),
		credentials.WithPassphrase(func() (string, error) { return passphrase, nil }),
	)
	require.NoError(t, err)
	return store
}

func TestEncryptedStore_SaveAndGet_RoundTrip(t *testing.T) {
	store := newTempEncryptedStore(t, "my-master-pass")

	want := credentials.Creds{Token: "xoxc-secret", Cookie: "xoxd-secret"}
	require.NoError(t, store.Save(context.Background(), "acme", want))

	got, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, want.Token, got.Token)
	assert.Equal(t, want.Cookie, got.Cookie)
}

func TestEncryptedStore_DataAtRestContainsNoPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	store, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) { return "pass", nil }),
	)
	require.NoError(t, err)
	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "xoxc-supersecret", Cookie: "xoxd-topsecret"}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.False(t, strings.Contains(string(raw), "xoxc-supersecret"), "token must not appear in plaintext on disk")
	assert.False(t, strings.Contains(string(raw), "xoxd-topsecret"), "cookie must not appear in plaintext on disk")
}

func TestEncryptedStore_WrongPassphraseFailsGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	writeStore, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) { return "correct-pass", nil }),
	)
	require.NoError(t, err)
	require.NoError(t, writeStore.Save(context.Background(), "ws", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"}))

	readStore, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) { return "wrong-pass", nil }),
	)
	require.NoError(t, err)

	_, err = readStore.Get(context.Background(), "ws")
	require.Error(t, err)
	assert.ErrorIs(t, err, credentials.ErrWrongPassphrase)
}

func TestEncryptedStore_PassphraseProviderCalledOnce(t *testing.T) {
	store := newTempEncryptedStore(t, "pass")
	require.NoError(t, store.Save(context.Background(), "ws", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"}))

	callCount := 0
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	writeStore, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) { return "pass", nil }),
	)
	require.NoError(t, err)
	require.NoError(t, writeStore.Save(context.Background(), "ws1", credentials.Creds{Token: "xoxc-1", Cookie: "xoxd-1"}))
	require.NoError(t, writeStore.Save(context.Background(), "ws2", credentials.Creds{Token: "xoxc-2", Cookie: "xoxd-2"}))

	readStore, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) {
			callCount++
			return "pass", nil
		}),
	)
	require.NoError(t, err)

	_, err = readStore.Get(context.Background(), "ws1")
	require.NoError(t, err)

	_, err = readStore.Get(context.Background(), "ws2")
	require.NoError(t, err)

	assert.Equal(t, 1, callCount, "passphrase provider must be called exactly once and then cached")
}

func TestEncryptedStore_PlaintextFileWithEncryptedStoreFails(t *testing.T) {
	// Write a plaintext store first.
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	plain, err := credentials.NewStoreAt(path)
	require.NoError(t, err)
	require.NoError(t, plain.Save(context.Background(), "ws", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"}))

	// Now open it with an encrypted store — must fail gracefully.
	enc, err := credentials.NewStoreAt(
		path,
		credentials.WithPassphrase(func() (string, error) { return "pass", nil }),
	)
	require.NoError(t, err)

	_, err = enc.Get(context.Background(), "ws")
	require.Error(t, err, "opening a plaintext file with an encrypted store must return an error")
}

func TestStore_MethodsAcceptContext(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, "ws", credentials.Creds{Token: "tok", Cookie: "ck"}))

	got, err := store.Get(ctx, "ws")
	require.NoError(t, err)
	assert.Equal(t, "tok", got.Token)

	names, err := store.List(ctx)
	require.NoError(t, err)
	assert.Contains(t, names, "ws")

	require.NoError(t, store.Delete(ctx, "ws"))

	_, err = store.Get(ctx, "ws")
	require.ErrorIs(t, err, credentials.ErrWorkspaceNotFound)
}
