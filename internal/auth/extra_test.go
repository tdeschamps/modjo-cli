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

func TestFingerprintNoUnderscore(t *testing.T) {
	// A long token without two underscores uses the 8-char prefix path.
	got := Fingerprint("abcdefghijklmnop")
	if got != "abcdefgh…mnop" {
		t.Errorf("Fingerprint = %q", got)
	}
}
