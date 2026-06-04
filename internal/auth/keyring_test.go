package auth

import (
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestKeyringStoreRoundTrip(t *testing.T) {
	keyring.MockInit() // in-memory keyring backend
	s := NewKeyringStore(t.TempDir() + "/fallback")

	cred := Credential{Token: "mjo_live_keyring_abcd", Method: MethodAPIKey, Workspace: "acme"}
	if err := s.Set("default", cred); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != cred.Token || got.Workspace != "acme" {
		t.Errorf("round-trip: %+v", got)
	}
	if err := s.Delete("default"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("default"); err != ErrNotFound {
		t.Errorf("after delete: %v", err)
	}
}

func TestKeyringFallsBackToFile(t *testing.T) {
	// Simulate an unavailable keychain so every op falls back to the file store.
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)
	defer keyring.MockInit()

	s := NewKeyringStore(t.TempDir() + "/fallback")
	cred := Credential{Token: "fallback-token", Method: MethodAPIKey}
	if err := s.Set("default", cred); err != nil {
		t.Fatalf("Set fallback: %v", err)
	}
	got, err := s.Get("default")
	if err != nil || got.Token != "fallback-token" {
		t.Fatalf("Get fallback: %+v %v", got, err)
	}
	if err := s.Delete("default"); err != nil {
		t.Fatalf("Delete fallback: %v", err)
	}
}

func TestKeyringGetFindsFileCredentialWhenKeychainEmpty(t *testing.T) {
	// A credential written by the file store (e.g. via MODJO_NO_KEYRING) must be
	// readable in normal keychain mode too: a healthy-but-empty keychain returns
	// ErrNotFound, and Get must then consult the file fallback instead of giving
	// up. Without this, logging in with MODJO_NO_KEYRING and then running a
	// command without it reports "not authenticated".
	keyring.MockInit() // keychain reachable but empty
	fallback := t.TempDir() + "/credentials"
	if err := NewFileStore(fallback).Set("default", Credential{Token: "file-token", Method: MethodAPIKey}); err != nil {
		t.Fatal(err)
	}
	s := NewKeyringStore(fallback)
	got, err := s.Get("default")
	if err != nil {
		t.Fatalf("Get should fall back to the file store, got %v", err)
	}
	if got.Token != "file-token" {
		t.Errorf("Get = %q, want file-token", got.Token)
	}
}

func TestKeyringGetTrulyMissing(t *testing.T) {
	// Empty keychain and empty file → ErrNotFound, not a fallback error.
	keyring.MockInit()
	s := NewKeyringStore(t.TempDir() + "/credentials")
	if _, err := s.Get("default"); err != ErrNotFound {
		t.Errorf("Get with nothing stored = %v, want ErrNotFound", err)
	}
}

func TestKeyringDeleteMissing(t *testing.T) {
	keyring.MockInit()
	s := NewKeyringStore(t.TempDir() + "/fallback")
	// Deleting a missing key falls through to the file fallback, which also
	// reports not found.
	if err := s.Delete("ghost"); err != ErrNotFound {
		t.Errorf("delete missing = %v", err)
	}
}

func TestKeyringSetDropsStaleFileCopy(t *testing.T) {
	keyring.MockInit()
	fallback := t.TempDir() + "/fallback"
	s := NewKeyringStore(fallback)

	// Seed a stale credential directly in the file fallback.
	file := NewFileStore(fallback)
	_ = file.Set("default", Credential{Token: "STALE-file-token"})

	// A successful keychain Set must clear the stale file copy so Get can't
	// later read the old one if the keychain becomes unavailable.
	if err := s.Set("default", Credential{Token: "fresh-keychain-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Get("default"); err != ErrNotFound {
		t.Error("stale file copy should have been dropped after a keychain Set")
	}
	got, _ := s.Get("default")
	if got.Token != "fresh-keychain-token" {
		t.Errorf("Get returned %q, want the fresh token", got.Token)
	}
}

func TestKeyringFallbackClearsStaleKeychain(t *testing.T) {
	// Keychain works for the first write, then becomes unavailable for the
	// second; the fallback write must not be shadowed by the stale keychain
	// entry on the next Get.
	keyring.MockInit()
	fallback := t.TempDir() + "/fallback"
	s := NewKeyringStore(fallback)
	if err := s.Set("default", Credential{Token: "old-keychain"}); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)
	defer keyring.MockInit()
	if err := s.Set("default", Credential{Token: "new-file"}); err != nil {
		t.Fatal(err)
	}
	// Keychain is down, so Get falls back to the file and reads the new token.
	got, err := s.Get("default")
	if err != nil || got.Token != "new-file" {
		t.Errorf("Get = %q (%v), want new-file", got.Token, err)
	}
}

func TestCredentialExpired(t *testing.T) {
	now := time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)
	if (Credential{}).Expired(now) {
		t.Error("zero expiry should never be expired")
	}
	past := Credential{Expiry: now.Add(-time.Hour)}
	if !past.Expired(now) {
		t.Error("past expiry should be expired")
	}
	future := Credential{Expiry: now.Add(time.Hour)}
	if future.Expired(now) {
		t.Error("future expiry should not be expired")
	}
}

func TestMemoryStoreDeleteMissing(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Delete("nope"); err != ErrNotFound {
		t.Errorf("delete missing = %v", err)
	}
}
