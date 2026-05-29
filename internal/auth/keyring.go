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

func (s *KeyringStore) Set(profile string, c Credential) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, profile, string(data)); err != nil {
		return s.fallback.Set(profile, c)
	}
	return nil
}

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
