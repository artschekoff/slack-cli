package credentials_test

import (
	"strings"
	"testing"

	"github.com/artschekoff/slack-cli/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	credentials.SetTestCryptoParams()
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	input := credentials.Creds{Token: "xoxc-abc123", Cookie: "xoxd-xyz789"}

	entry, err := credentials.EncryptCreds("my-secret-passphrase", input)
	require.NoError(t, err)

	got, err := credentials.DecryptCreds("my-secret-passphrase", entry)
	require.NoError(t, err)

	assert.Equal(t, input.Token, got.Token)
	assert.Equal(t, input.Cookie, got.Cookie)
}

func TestEncryptDecrypt_WrongPassphraseReturnsError(t *testing.T) {
	entry, err := credentials.EncryptCreds("correct-pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	_, err = credentials.DecryptCreds("wrong-pass", entry)
	require.Error(t, err)
	assert.ErrorIs(t, err, credentials.ErrWrongPassphrase)
}

func TestEncryptDecrypt_EmptyPassphraseReturnsError(t *testing.T) {
	_, err := credentials.EncryptCreds("", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase")
}

func TestEncryptDecrypt_DifferentEntriesForSameCreds(t *testing.T) {
	creds := credentials.Creds{Token: "xoxc-same", Cookie: "xoxd-same"}

	entry1, err := credentials.EncryptCreds("pass", creds)
	require.NoError(t, err)

	entry2, err := credentials.EncryptCreds("pass", creds)
	require.NoError(t, err)

	// Random salt and nonce mean ciphertexts differ on every call.
	assert.NotEqual(t, entry1.Data, entry2.Data, "same plaintext must produce different ciphertexts")
	assert.NotEqual(t, entry1.Salt, entry2.Salt, "each encryption must use a fresh random salt")
}

func TestEncryptDecrypt_CiphertextContainsNoPlaintext(t *testing.T) {
	creds := credentials.Creds{Token: "xoxc-supersecret", Cookie: "xoxd-topsecret"}

	entry, err := credentials.EncryptCreds("passphrase", creds)
	require.NoError(t, err)

	assert.False(t, strings.Contains(entry.Data, "xoxc-supersecret"), "ciphertext must not contain token in plaintext")
	assert.False(t, strings.Contains(entry.Data, "xoxd-topsecret"), "ciphertext must not contain cookie in plaintext")
}

func TestEncryptDecrypt_TamperedCiphertextReturnsError(t *testing.T) {
	entry, err := credentials.EncryptCreds("pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	tampered := entry
	// Flip a character near the end of the base64 data.
	data := []byte(tampered.Data)
	data[len(data)-2] ^= 0xff
	tampered.Data = string(data)

	_, err = credentials.DecryptCreds("pass", tampered)
	require.Error(t, err)
	assert.ErrorIs(t, err, credentials.ErrWrongPassphrase)
}

// --- Argon2 parameter validation tests (security: tampered credentials file) ---

func TestDecryptCreds_ZeroMemoryRejected(t *testing.T) {
	entry, err := credentials.EncryptCreds("pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	entry.Memory = 0

	_, err = credentials.DecryptCreds("pass", entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tampered")
}

func TestDecryptCreds_ZeroTimeRejected(t *testing.T) {
	entry, err := credentials.EncryptCreds("pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	entry.Time = 0

	_, err = credentials.DecryptCreds("pass", entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tampered")
}

func TestDecryptCreds_ZeroThreadsRejected(t *testing.T) {
	entry, err := credentials.EncryptCreds("pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	entry.Threads = 0

	_, err = credentials.DecryptCreds("pass", entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tampered")
}

func TestDecryptCreds_MemoryBelowMinimumRejected(t *testing.T) {
	entry, err := credentials.EncryptCreds("pass", credentials.Creds{Token: "xoxc-t", Cookie: "xoxd-c"})
	require.NoError(t, err)

	// Set Memory to 1 KiB — far below the security floor even in test mode.
	entry.Memory = 1

	_, err = credentials.DecryptCreds("pass", entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tampered")
}

func TestDecryptCreds_ValidParamsSucceed(t *testing.T) {
	creds := credentials.Creds{Token: "xoxc-valid", Cookie: "xoxd-valid"}
	entry, err := credentials.EncryptCreds("pass", creds)
	require.NoError(t, err)

	// Entry produced by EncryptCreds uses the current activeParams which are
	// always >= minParams, so decryption must succeed.
	got, err := credentials.DecryptCreds("pass", entry)
	require.NoError(t, err)
	assert.Equal(t, creds.Token, got.Token)
	assert.Equal(t, creds.Cookie, got.Cookie)
}
