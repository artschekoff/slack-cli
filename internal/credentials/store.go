// Package credentials manages Slack workspace credentials stored in ~/.slack/workspace_credentials.json.
package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	defaultCredDir  = ".slack"
	defaultCredFile = "workspace_credentials.json"
)

// ErrWorkspaceNotFound is returned when a workspace key is not present in the store.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// Creds holds the Slack token and browser cookie for a single workspace.
type Creds struct {
	Token  string `json:"token"`
	Cookie string `json:"cookie"`
}

// PassphraseProvider returns the passphrase used to encrypt and decrypt credentials.
// It is called at most once per Store instance; the result is cached in memory.
type PassphraseProvider func() (string, error)

// Option configures a Store.
type Option func(*Store)

// WithPassphrase sets a PassphraseProvider on the Store, enabling AES-256-GCM
// encryption of all credentials at rest. When this option is supplied the on-disk
// format changes from plain JSON to a map of EncryptedEntry objects.
func WithPassphrase(p PassphraseProvider) Option {
	return func(s *Store) {
		s.passphraseProvider = p
	}
}

// Store is a thread-safe credential store backed by a JSON file.
//
// When created with WithPassphrase, credentials are encrypted at rest using
// AES-256-GCM with a key derived from the passphrase via argon2id. The
// passphrase is requested from the provider at most once; the result is cached
// for the lifetime of the Store.
type Store struct {
	mu       sync.RWMutex
	filePath string

	passphraseProvider PassphraseProvider
	passphraseMu       sync.Mutex
	cachedPassphrase   string
	passphraseFetched  bool
}

// NewStore creates a Store using the default path (~/.slack/workspace_credentials.json).
func NewStore(opts ...Option) (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	return NewStoreAt(filepath.Join(home, defaultCredDir, defaultCredFile), opts...)
}

// NewStoreAt creates a Store at the given file path, creating parent directories as needed.
func NewStoreAt(path string, opts ...Option) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating credentials directory: %w", err)
	}

	s := &Store{filePath: path}
	for _, opt := range opts {
		opt(s)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := s.writeFile(path, map[string]json.RawMessage{}); err != nil {
			return nil, fmt.Errorf("initializing credentials file: %w", err)
		}
	}

	return s, nil
}

// List returns the names of all saved workspaces.
func (s *Store) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := s.readRawAll()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(raw))
	for k := range raw {
		names = append(names, k)
	}
	return names, nil
}

// Path returns the absolute path to the credentials file.
func (s *Store) Path() string {
	return s.filePath
}

// Get returns credentials for the given workspace, or ErrWorkspaceNotFound.
func (s *Store) Get(_ context.Context, workspace string) (Creds, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := s.readRawAll()
	if err != nil {
		return Creds{}, err
	}

	entry, ok := raw[workspace]
	if !ok {
		return Creds{}, ErrWorkspaceNotFound
	}

	return s.decodeEntry(entry)
}

// Save writes credentials for the given workspace, creating or overwriting the entry.
func (s *Store) Save(_ context.Context, workspace string, creds Creds) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := s.readRawAll()
	if err != nil {
		return err
	}

	encoded, err := s.encodeEntry(creds)
	if err != nil {
		return err
	}

	raw[workspace] = encoded
	return s.writeFile(s.filePath, raw)
}

// Delete removes the workspace entry. Returns ErrWorkspaceNotFound if absent.
func (s *Store) Delete(_ context.Context, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := s.readRawAll()
	if err != nil {
		return err
	}

	if _, ok := raw[workspace]; !ok {
		return ErrWorkspaceNotFound
	}

	delete(raw, workspace)
	return s.writeFile(s.filePath, raw)
}

// passphrase returns the cached passphrase, calling the provider once if needed.
func (s *Store) passphrase() (string, error) {
	s.passphraseMu.Lock()
	defer s.passphraseMu.Unlock()

	if s.passphraseFetched {
		return s.cachedPassphrase, nil
	}

	p, err := s.passphraseProvider()
	if err != nil {
		return "", fmt.Errorf("fetching passphrase: %w", err)
	}

	s.cachedPassphrase = p
	s.passphraseFetched = true
	return p, nil
}

// encodeEntry serialises creds, encrypting when a PassphraseProvider is set.
func (s *Store) encodeEntry(creds Creds) (json.RawMessage, error) {
	if s.passphraseProvider == nil {
		return json.Marshal(creds)
	}

	pass, err := s.passphrase()
	if err != nil {
		return nil, err
	}

	entry, err := EncryptCreds(pass, creds)
	if err != nil {
		return nil, fmt.Errorf("encrypting credentials: %w", err)
	}

	return json.Marshal(entry)
}

// decodeEntry deserialises a raw JSON blob back into Creds.
// In encrypted mode it also decrypts; in plaintext mode it unmarshals directly.
// If the file was written in plaintext but the store expects encryption, an
// informative error is returned instead of silently returning garbage.
func (s *Store) decodeEntry(raw json.RawMessage) (Creds, error) {
	if s.passphraseProvider == nil {
		var creds Creds
		if err := json.Unmarshal(raw, &creds); err != nil {
			return Creds{}, fmt.Errorf("parsing credentials: %w", err)
		}
		return creds, nil
	}

	// Encrypted mode: expect an EncryptedEntry.
	var entry EncryptedEntry
	if err := json.Unmarshal(raw, &entry); err != nil || entry.Data == "" {
		return Creds{}, fmt.Errorf("credentials are stored in plaintext but encryption is enabled; re-run 'auth' to re-authenticate")
	}

	pass, err := s.passphrase()
	if err != nil {
		return Creds{}, err
	}

	return DecryptCreds(pass, entry)
}

func (s *Store) readRawAll() (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("parsing credentials file: %w", err)
	}
	if all == nil {
		all = map[string]json.RawMessage{}
	}
	return all, nil
}

func (s *Store) writeFile(path string, all map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising credentials: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".creds-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting permissions on temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing credentials file: %w", err)
	}
	return nil
}
