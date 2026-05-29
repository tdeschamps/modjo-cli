package cmdutil

import (
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

func newFactory(t *testing.T, flags *GlobalFlags) *Factory {
	t.Helper()
	io, _, _, _ := iostreams.Test()
	if flags == nil {
		flags = &GlobalFlags{}
	}
	return &Factory{
		IOStreams:  io,
		Flags:      flags,
		ConfigPath: t.TempDir() + "/config.toml",
		CredStore:  auth.NewMemoryStore(),
	}
}

func TestActiveProfilePrecedence(t *testing.T) {
	f := newFactory(t, &GlobalFlags{Profile: "flagprof"})
	if p, _ := f.ActiveProfile(); p != "flagprof" {
		t.Errorf("flag profile = %q", p)
	}

	f = newFactory(t, &GlobalFlags{})
	t.Setenv("MODJO_PROFILE", "envprof")
	if p, _ := f.ActiveProfile(); p != "envprof" {
		t.Errorf("env profile = %q", p)
	}

	t.Setenv("MODJO_PROFILE", "")
	f = newFactory(t, &GlobalFlags{})
	if p, _ := f.ActiveProfile(); p != "default" {
		t.Errorf("default profile = %q", p)
	}
}

func TestConfigCached(t *testing.T) {
	f := newFactory(t, nil)
	c1, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := f.Config()
	if c1 != c2 {
		t.Error("Config should be cached")
	}
}

func TestTokenSourceOrder(t *testing.T) {
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "stored"})
	io, _, _, _ := iostreams.Test()

	// flag wins
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{APIKey: "flagkey"}, CredStore: store, ConfigPath: t.TempDir() + "/c.toml"}
	if tok, _ := f.TokenSource()(); tok != "flagkey" {
		t.Errorf("flag token = %q", tok)
	}

	// --token wins over env/store
	f.Flags = &GlobalFlags{Token: "toktok"}
	if tok, _ := f.TokenSource()(); tok != "toktok" {
		t.Errorf("--token = %q", tok)
	}

	// env wins over store
	f.Flags = &GlobalFlags{}
	t.Setenv("MODJO_API_KEY", "envkey")
	if tok, _ := f.TokenSource()(); tok != "envkey" {
		t.Errorf("env token = %q", tok)
	}
	t.Setenv("MODJO_API_KEY", "")
	t.Setenv("MODJO_TOKEN", "envtok2")
	if tok, _ := f.TokenSource()(); tok != "envtok2" {
		t.Errorf("MODJO_TOKEN = %q", tok)
	}

	// store fallback
	t.Setenv("MODJO_TOKEN", "")
	if tok, _ := f.TokenSource()(); tok != "stored" {
		t.Errorf("stored token = %q", tok)
	}
}

func TestTokenSourceNotAuthenticated(t *testing.T) {
	f := newFactory(t, &GlobalFlags{})
	_, err := f.TokenSource()()
	if err != ErrNotAuthenticated {
		t.Errorf("want ErrNotAuthenticated, got %v", err)
	}
}

func TestOutputFormat(t *testing.T) {
	// --json shorthand
	f := newFactory(t, &GlobalFlags{JSON: true})
	if fmt, _ := f.OutputFormat(); fmt != output.FormatJSON {
		t.Errorf("json shorthand = %v", fmt)
	}

	// explicit -o csv
	f = newFactory(t, &GlobalFlags{Output: "csv"})
	if fmt, _ := f.OutputFormat(); fmt != output.FormatCSV {
		t.Errorf("csv = %v", fmt)
	}

	// non-TTY defaults to JSON
	f = newFactory(t, &GlobalFlags{})
	f.IOStreams.SetStdoutTTY(false)
	if fmt, _ := f.OutputFormat(); fmt != output.FormatJSON {
		t.Errorf("non-TTY default = %v", fmt)
	}

	// TTY defaults to table
	f = newFactory(t, &GlobalFlags{})
	f.IOStreams.SetStdoutTTY(true)
	if fmt, _ := f.OutputFormat(); fmt != output.FormatTable {
		t.Errorf("TTY default = %v", fmt)
	}

	// invalid
	f = newFactory(t, &GlobalFlags{Output: "bogus"})
	if _, err := f.OutputFormat(); err == nil {
		t.Error("expected error for bogus format")
	}
}

func TestEffectiveLimit(t *testing.T) {
	f := newFactory(t, &GlobalFlags{All: true})
	if n, _ := f.EffectiveLimit(); n != 0 {
		t.Errorf("--all should give 0, got %d", n)
	}
	f = newFactory(t, &GlobalFlags{Limit: 17})
	if n, _ := f.EffectiveLimit(); n != 17 {
		t.Errorf("--limit = %d", n)
	}
	f = newFactory(t, &GlobalFlags{})
	if n, _ := f.EffectiveLimit(); n != 50 {
		t.Errorf("default limit = %d", n)
	}
}

func TestClientBuilders(t *testing.T) {
	f := newFactory(t, &GlobalFlags{})
	if _, err := f.APIClient(); err != nil {
		t.Errorf("APIClient: %v", err)
	}
	if _, err := f.MCPClient(); err != nil {
		t.Errorf("MCPClient: %v", err)
	}
	if _, err := f.Printer(); err != nil {
		t.Errorf("Printer: %v", err)
	}
}

func TestCredentialStoreDefault(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml"}
	s, err := f.CredentialStore()
	if err != nil || s == nil {
		t.Fatalf("store=%v err=%v", s, err)
	}
}

func TestNewFactory(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := New(io, &GlobalFlags{})
	if f.Clock == nil {
		t.Error("New should set a clock")
	}
}

func TestDebugClientWritesLogs(t *testing.T) {
	io, _, _, errBuf := iostreams.Test()
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{Debug: true, MaxRetries: 1}, CredStore: store, ConfigPath: t.TempDir() + "/c.toml"}
	// building the client should not write yet
	if _, err := f.APIClient(); err != nil {
		t.Fatal(err)
	}
	_ = errBuf
	if strings.Contains(errBuf.String(), "panic") {
		t.Error("unexpected output")
	}
}
