// Package session persists the master passphrase used to decrypt Slack
// workspace credentials in an AES-256-GCM encrypted file on disk.
//
// The symmetric key is derived from stable machine + user identifiers
// (hostname, home directory, a version tag). This protects the passphrase
// from casual disk inspection and cloud-backup exposure — it does NOT
// protect against local malware running as the same user.
package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoSession is returned by Load when the session file does not exist.
var ErrNoSession = errors.New("no active session")

const keyDerivationTag = "slack-cli-session-v1"

// Store persists a single passphrase, encrypted at rest.
type Store struct {
	path string
}

// NewStore constructs a Store backed by the file at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the standard session file location: ~/.slack/session.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".slack", "session"), nil
}

// Path returns the file path this store reads and writes.
func (s *Store) Path() string { return s.path }

// Save encrypts passphrase and writes it to disk with mode 0600,
// creating the parent directory with mode 0700 if missing.
func (s *Store) Save(passphrase string) error {
	if passphrase == "" {
		return errors.New("passphrase must not be empty")
	}
	ct, err := encrypt([]byte(passphrase))
	if err != nil {
		return fmt.Errorf("encrypting session: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}
	if err := os.WriteFile(s.path, ct, 0o600); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	return nil
}

// Load reads and decrypts the passphrase. Returns ErrNoSession when the
// file does not exist. Any other error (tampering, wrong key) is wrapped.
func (s *Store) Load() (string, error) {
	ct, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoSession
		}
		return "", fmt.Errorf("reading session file: %w", err)
	}
	pt, err := decrypt(ct)
	if err != nil {
		return "", fmt.Errorf("decrypting session: %w", err)
	}
	return string(pt), nil
}

// Delete removes the session file. Missing file is not an error.
func (s *Store) Delete() error {
	if err := os.Remove(s.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("removing session file: %w", err)
	}
	return nil
}

// sessionKey derives a 32-byte symmetric key from stable local identifiers.
// ponytail: this is obfuscation-at-rest, not a defense against local malware
// under the same user account; upgrade to OS keychain if that threat model changes.
func sessionKey() [32]byte {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	seed := hostname + "|" + home + "|" + keyDerivationTag
	return sha256.Sum256([]byte(seed))
}

func encrypt(plaintext []byte) ([]byte, error) {
	key := sessionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte) ([]byte, error) {
	key := sessionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("session file too short")
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
