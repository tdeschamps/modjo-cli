package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreLoadErrors(t *testing.T) {
	dir := t.TempDir()

	// Unparseable JSON → error on Get.
	path := filepath.Join(dir, "creds")
	_ = os.WriteFile(path, []byte("{not json"), 0o600)
	s := NewFileStore(path)
	if _, err := s.Get("default"); err == nil {
		t.Error("expected JSON parse error")
	}
	if err := s.Set("default", Credential{Token: "t"}); err == nil {
		t.Error("Set should fail when existing file is unparseable")
	}
	if err := s.Delete("default"); err == nil {
		t.Error("Delete should fail when existing file is unparseable")
	}
}

func TestFileStoreEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds")
	_ = os.WriteFile(path, []byte(""), 0o600)
	s := NewFileStore(path)
	if _, err := s.Get("default"); err != ErrNotFound {
		t.Errorf("empty file Get = %v", err)
	}
}

func TestFileStoreSaveMkdirError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "afile")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	// Parent is a file → MkdirAll fails.
	s := NewFileStore(filepath.Join(file, "sub", "creds"))
	if err := s.Set("default", Credential{Token: "t"}); err == nil {
		t.Error("expected mkdir error")
	}
}

func TestFingerprintNeverLeaksBody(t *testing.T) {
	// A token without a recognizable scheme prefix must reveal only the last
	// four characters — never the body (regression for the full-secret leak).
	for _, tok := range []string{
		"abcdefghijklmnop",      // 16 chars, no underscore
		"secrettoken9",          // 12 chars
		"sk-thisIsASecretValue", // dash prefix, not underscore-scheme
	} {
		got := Fingerprint(tok)
		body := tok[:len(tok)-4]
		if got != "…"+tok[len(tok)-4:] {
			t.Errorf("Fingerprint(%q) = %q, want %q", tok, got, "…"+tok[len(tok)-4:])
		}
		// Defense in depth: the masked output must not contain the secret body.
		if len(body) > 4 && containsSub(got, body) {
			t.Errorf("Fingerprint(%q) leaked the body: %q", tok, got)
		}
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
