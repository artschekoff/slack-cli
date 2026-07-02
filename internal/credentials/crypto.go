package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen  = 16
	nonceLen = 12
	keyLen   = 32 // AES-256
)

// ErrWrongPassphrase is returned when decryption fails due to an incorrect
// passphrase or corrupted ciphertext.
var ErrWrongPassphrase = errors.New("wrong passphrase or corrupted data")

// EncryptedEntry is the on-disk representation of one encrypted workspace credential.
type EncryptedEntry struct {
	Data    string `json:"data"`    // base64(AES-256-GCM ciphertext + GCM tag)
	Salt    string `json:"salt"`    // base64(argon2 salt, 16 bytes)
	Nonce   string `json:"nonce"`   // base64(GCM nonce, 12 bytes)
	Memory  uint32 `json:"memory"`  // argon2 memory parameter in KiB
	Time    uint32 `json:"time"`    // argon2 time parameter (iterations)
	Threads uint8  `json:"threads"` // argon2 parallelism
}

// cryptoParams holds argon2id tuning values.
// Production defaults are applied by newEntry; tests inject lower params.
type cryptoParams struct {
	Memory  uint32
	Time    uint32
	Threads uint8
}

var productionParams = cryptoParams{
	Memory:  64 * 1024, // 64 MiB
	Time:    1,
	Threads: 4,
}

// activeParams is used for all encrypt operations.
// Tests override this via setTestCryptoParams (export_test.go).
var activeParams = productionParams

// EncryptCreds encrypts creds with AES-256-GCM using a key derived from
// passphrase via argon2id. Each call produces a unique ciphertext because a
// fresh random salt and nonce are generated.
func EncryptCreds(passphrase string, creds Creds) (EncryptedEntry, error) {
	if passphrase == "" {
		return EncryptedEntry{}, errors.New("passphrase must not be empty")
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return EncryptedEntry{}, fmt.Errorf("generating salt: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedEntry{}, fmt.Errorf("generating nonce: %w", err)
	}

	p := activeParams
	key := deriveKey(passphrase, salt, p)

	gcm, err := newGCM(key)
	if err != nil {
		return EncryptedEntry{}, err
	}

	plaintext, err := json.Marshal(creds)
	if err != nil {
		return EncryptedEntry{}, fmt.Errorf("marshalling credentials: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return EncryptedEntry{
		Data:    base64.StdEncoding.EncodeToString(ciphertext),
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Memory:  p.Memory,
		Time:    p.Time,
		Threads: p.Threads,
	}, nil
}

// DecryptCreds decrypts an EncryptedEntry using the given passphrase.
// Returns ErrWrongPassphrase if the passphrase is incorrect or the data is corrupt.
func DecryptCreds(passphrase string, entry EncryptedEntry) (Creds, error) {
	if entry.Memory < 8 || entry.Time < 1 || entry.Threads < 1 {
		return Creds{}, fmt.Errorf("tampered or invalid argon2 parameters")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(entry.Data)
	if err != nil {
		return Creds{}, ErrWrongPassphrase
	}

	salt, err := base64.StdEncoding.DecodeString(entry.Salt)
	if err != nil {
		return Creds{}, ErrWrongPassphrase
	}

	nonce, err := base64.StdEncoding.DecodeString(entry.Nonce)
	if err != nil {
		return Creds{}, ErrWrongPassphrase
	}

	p := cryptoParams{Memory: entry.Memory, Time: entry.Time, Threads: entry.Threads}
	key := deriveKey(passphrase, salt, p)

	gcm, err := newGCM(key)
	if err != nil {
		return Creds{}, fmt.Errorf("creating cipher: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return Creds{}, ErrWrongPassphrase
	}

	var creds Creds
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return Creds{}, ErrWrongPassphrase
	}

	return creds, nil
}

func deriveKey(passphrase string, salt []byte, p cryptoParams) []byte {
	return argon2.IDKey([]byte(passphrase), salt, p.Time, p.Memory, p.Threads, keyLen)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	return gcm, nil
}
