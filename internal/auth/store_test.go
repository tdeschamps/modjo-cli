package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewFileStore(path)

	cred := Credential{
		Token:     "mjo_live_secret_value_a91f",
		Method:    MethodAPIKey,
		Workspace: "acme-eu",
		Scopes:    []string{"calls:read", "deals:read"},
		Expiry:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := s.Set("default", cred); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != cred.Token || got.Workspace != "acme-eu" || got.Method != MethodAPIKey {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if len(got.Scopes) != 2 {
		t.Errorf("scopes lost: %+v", got.Scopes)
	}

	if err := s.Delete("default"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("default"); err != ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestFileStoreIsolatesProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewFileStore(path)
	_ = s.Set("a", Credential{Token: "tok-a", Method: MethodAPIKey})
	_ = s.Set("b", Credential{Token: "tok-b", Method: MethodAPIKey})

	a, _ := s.Get("a")
	b, _ := s.Get("b")
	if a.Token != "tok-a" || b.Token != "tok-b" {
		t.Errorf("profiles bled together: a=%q b=%q", a.Token, b.Token)
	}
}

func TestFingerprint(t *testing.T) {
	cases := []struct{ in, want string }{
		{"mjo_live_1234567890abcdef", "mjo_live_…cdef"},
		{"short", "…"},
		{"", ""},
		// No recognizable scheme prefix: reveal only the last four, never the body.
		{"secrettoken9", "…ken9"},     // 12 chars, no 2nd underscore
		{"123456789012", "…9012"},     // 12 chars, digits only
		{"abcdefghijklmnop", "…mnop"}, // 16 chars, no underscore
		{"abcdefgh", "…"},             // exactly 8 → fully redacted
	}
	for _, tc := range cases {
		if got := Fingerprint(tc.in); got != tc.want {
			t.Errorf("Fingerprint(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
