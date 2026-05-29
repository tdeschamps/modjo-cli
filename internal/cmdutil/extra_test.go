package cmdutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func TestErrorTypes(t *testing.T) {
	ue := NewUsageError(errors.New("bad"))
	if ue.Error() != "bad" || ue.Unwrap().Error() != "bad" {
		t.Errorf("UsageError: %v / %v", ue.Error(), ue.Unwrap())
	}
	se := NewSilentError(7, errors.New("quiet"))
	if se.Error() != "quiet" || se.Unwrap().Error() != "quiet" || se.Code != 7 {
		t.Errorf("SilentError: %v / %v / %d", se.Error(), se.Unwrap(), se.Code)
	}
}

func TestPromptSecretTTYPath(t *testing.T) {
	orig := readPasswordTTY
	readPasswordTTY = func(io *iostreams.IOStreams) (string, bool, error) {
		return "tty-secret", true, nil
	}
	defer func() { readPasswordTTY = orig }()

	io, _, _, _ := iostreams.Test()
	got, err := PromptSecret(io, "key: ")
	if err != nil || got != "tty-secret" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestPromptSecretTTYError(t *testing.T) {
	orig := readPasswordTTY
	readPasswordTTY = func(io *iostreams.IOStreams) (string, bool, error) {
		return "", true, errors.New("read fail")
	}
	defer func() { readPasswordTTY = orig }()

	io, _, _, _ := iostreams.Test()
	if _, err := PromptSecret(io, "key: "); err == nil {
		t.Fatal("expected error from TTY read")
	}
}

// withBadConfig points the factory at an unparseable config file to exercise
// the error branches that depend on Config() failing.
func badConfigFactory(t *testing.T) *Factory {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("this is = = not toml ]["), 0o600); err != nil {
		t.Fatal(err)
	}
	io, _, _, _ := iostreams.Test()
	return &Factory{IOStreams: io, Flags: &GlobalFlags{}, ConfigPath: path}
}

func TestConfigErrorPropagates(t *testing.T) {
	f := badConfigFactory(t)
	if _, err := f.Config(); err == nil {
		t.Fatal("expected config parse error")
	}

	for name, fn := range map[string]func() error{
		"ActiveProfile":  func() error { _, e := f.ActiveProfile(); return e },
		"Resolver":       func() error { _, e := f.Resolver(); return e },
		"APIClient":      func() error { _, e := f.APIClient(); return e },
		"MCPClient":      func() error { _, e := f.MCPClient(); return e },
		"OutputFormat":   func() error { _, e := f.OutputFormat(); return e },
		"EffectiveLimit": func() error { _, e := f.EffectiveLimit(); return e },
		"Printer":        func() error { _, e := f.Printer(); return e },
	} {
		if err := fn(); err == nil {
			t.Errorf("%s should propagate config error", name)
		}
	}
}

func TestExitCodeForErrorMore(t *testing.T) {
	if got := ExitCodeForError(NewSilentError(9, errors.New("x"))); got != 9 {
		t.Errorf("silent code = %d", got)
	}
}

func TestSaveConfigDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("MODJO_CONFIG", "")
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{}}
	cfg, _ := f.Config()
	if err := f.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig default path: %v", err)
	}
}

func TestRenderBadFormatBranches(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{Output: "bogus"}, ConfigPath: t.TempDir() + "/c.toml"}
	if err := CollectAndRender(t.Context(), f, seqOf([]row{{"a"}}, nil), fields()); err == nil {
		t.Error("CollectAndRender should error on bad format")
	}
	if err := RenderOne(f, row{"a"}, fields()); err == nil {
		t.Error("RenderOne should error on bad format")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func TestConfirmReadError(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	io.SetNeverPrompt(false)
	io.In = errReader{}
	if _, err := Confirm(io, "ok?", false); err == nil {
		t.Error("expected read error")
	}
}

func TestExitCodeUnmappedAPIError(t *testing.T) {
	if got := ExitCodeForError(&api.Error{StatusCode: 400}); got != ExitError {
		t.Errorf("unmapped 4xx = %d want %d", got, ExitError)
	}
}

func TestExitCodeNotAuthenticated(t *testing.T) {
	if got := ExitCodeForError(ErrNotAuthenticated); got != ExitAuth {
		t.Errorf("not-authenticated = %d want %d", got, ExitAuth)
	}
}

func TestResolverFlagProfile(t *testing.T) {
	f := newFactory(t, &GlobalFlags{Profile: "myprof"})
	r, err := f.Resolver()
	if err != nil || r.Profile != "myprof" {
		t.Fatalf("resolver profile = %q err=%v", r.Profile, err)
	}
}

// withNoHome makes config.DefaultPath fail so the DefaultPath error branches run.
func withNoHome(t *testing.T) {
	t.Helper()
	t.Setenv("MODJO_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("APPDATA", "")
}

func TestDefaultPathFailureBranches(t *testing.T) {
	withNoHome(t)
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{}}
	if _, err := f.Config(); err == nil {
		t.Error("Config should fail when home is undiscoverable")
	}
	if _, err := f.CredentialStore(); err == nil {
		t.Error("CredentialStore should fail when home is undiscoverable")
	}
	if err := f.SaveConfig(nil); err == nil {
		t.Error("SaveConfig should fail when home is undiscoverable")
	}
}

func TestTokenSourceStoreError(t *testing.T) {
	withNoHome(t)
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{}}
	if _, err := f.TokenSource()(); err == nil {
		t.Error("TokenSource should surface credential store error")
	}
}

func TestDefaultPathBranches(t *testing.T) {
	// ConfigPath empty → uses DefaultPath via HOME.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("MODJO_CONFIG", "")
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{}}
	if _, err := f.Config(); err != nil {
		t.Fatalf("Config with default path: %v", err)
	}
}
