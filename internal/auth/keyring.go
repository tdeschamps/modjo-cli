package auth

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

const keyringService = "modjo-cli"

// KeyringStore stores each profile's credential in the OS keychain. If the
// keychain is unavailable (headless Linux, CI), it transparently falls back to
// the provided FileStore so the CLI still works everywhere.
type KeyringStore struct {
	fallback *FileStore
}

// NewKeyringStore returns a keychain-backed store with a file fallback.
func NewKeyringStore(fallbackPath string) *KeyringStore {
	return &KeyringStore{fallback: NewFileStore(fallbackPath)}
}

// Get returns the credential for profile from the keychain, falling back to the
// file store when the keychain is unavailable.
func (s *KeyringStore) Get(profile string) (Credential, error) {
	raw, err := keyring.Get(keyringService, profile)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Credential{}, ErrNotFound
		}
		return s.fallback.Get(profile)
	}
	var c Credential
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return Credential{}, err
	}
	return c, nil
}

// Set stores c for profile in the keychain, falling back to the file store.
//
// The two backends are kept mutually exclusive so a stale copy in one can never
// shadow a fresh write to the other: on a successful keychain write we drop any
// file copy, and when we fall back to the file we drop any keychain copy. Get
// reads the keychain first, so without this a transient keychain.Set failure
// would leave an old token in the keychain that Get would keep returning.
func (s *KeyringStore) Set(profile string, c Credential) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, profile, string(data)); err != nil {
		// Keychain unavailable: persist to file and best-effort clear any stale
		// keychain entry so it can't shadow this write on the next Get.
		_ = keyring.Delete(keyringService, profile)
		return s.fallback.Set(profile, c)
	}
	// Stored in the keychain: best-effort drop any stale file copy.
	_ = s.fallback.Delete(profile)
	return nil
}

// Delete removes the credential for profile from the keychain (and file store).
func (s *KeyringStore) Delete(profile string) error {
	err := keyring.Delete(keyringService, profile)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			// Maybe it lives in the fallback.
			return s.fallback.Delete(profile)
		}
		return s.fallback.Delete(profile)
	}
	return nil
}
