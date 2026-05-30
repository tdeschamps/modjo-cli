// Package auth owns credential modeling and storage. Credentials are kept out
// of the main config file: in the OS keychain when available, otherwise in a
// 0600 file. Secrets are never logged and only ever surfaced as masked
// fingerprints (see Fingerprint).
package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Auth methods.
const (
	MethodAPIKey = "api_key"
	MethodOAuth  = "oauth"
)

// ErrNotFound is returned when no credential exists for a profile.
var ErrNotFound = errors.New("no stored credential for profile")

// Credential is a stored secret plus its metadata.
type Credential struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Method       string    `json:"method"`
	Workspace    string    `json:"workspace,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

// Expired reports whether an OAuth credential has passed its expiry.
func (c Credential) Expired(now time.Time) bool {
	if c.Expiry.IsZero() {
		return false
	}
	return !now.Before(c.Expiry)
}

// Store abstracts credential persistence so the backend (keychain vs file) and
// test fakes are interchangeable.
type Store interface {
	Get(profile string) (Credential, error)
	Set(profile string, c Credential) error
	Delete(profile string) error
}

// MemoryStore is an in-memory Store for tests.
type MemoryStore struct{ m map[string]Credential }

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]Credential{}} }

// Get returns the credential stored for profile.
func (s *MemoryStore) Get(profile string) (Credential, error) {
	c, ok := s.m[profile]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return c, nil
}

// Set stores c under profile.
func (s *MemoryStore) Set(profile string, c Credential) error { s.m[profile] = c; return nil }

// Delete removes the credential for profile.
func (s *MemoryStore) Delete(profile string) error {
	if _, ok := s.m[profile]; !ok {
		return ErrNotFound
	}
	delete(s.m, profile)
	return nil
}

// FileStore persists credentials as a single 0600 JSON file keyed by profile.
type FileStore struct{ path string }

// NewFileStore returns a FileStore backed by path.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

func (s *FileStore) load() (map[string]Credential, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]Credential{}
	if len(data) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *FileStore) save(m map[string]Credential) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get returns the credential stored for profile.
func (s *FileStore) Get(profile string) (Credential, error) {
	m, err := s.load()
	if err != nil {
		return Credential{}, err
	}
	c, ok := m[profile]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return c, nil
}

// Set stores c under profile, preserving other profiles' credentials.
func (s *FileStore) Set(profile string, c Credential) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	m[profile] = c
	return s.save(m)
}

// Delete removes the credential for profile.
func (s *FileStore) Delete(profile string) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := m[profile]; !ok {
		return ErrNotFound
	}
	delete(m, profile)
	return s.save(m)
}

// Fingerprint masks a secret for display: it always hides the body of the
// token, revealing only a recognizable scheme prefix (e.g. "mjo_live_") when
// present and the last four characters — like "mjo_live_…a91f" or "…a91f".
// Short or empty values are fully redacted. It never reveals the secret body,
// regardless of token shape.
func Fingerprint(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "…"
	}
	last4 := token[len(token)-4:]
	// Only reveal a leading segment when it's a recognizable, non-secret scheme
	// prefix (up to a second underscore that sits before the last four chars).
	prefix := ""
	if i := nthIndex(token, "_", 2); i > 0 && i < len(token)-4 {
		prefix = token[:i+1]
	}
	return prefix + "…" + last4
}

func nthIndex(s, sub string, n int) int {
	idx := -1
	for i := 0; i < n; i++ {
		j := strings.Index(s[idx+1:], sub)
		if j < 0 {
			return -1
		}
		idx += j + 1
	}
	return idx
}
