package cmdutil

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

func mustNow() time.Time {
	return time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)
}

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{"https://x"}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", "https://x"}},
		{"linux", "xdg-open", []string{"https://x"}},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			name, args := browserCommand(tc.goos, "https://x")
			if name != tc.wantName {
				t.Errorf("name = %q want %q", name, tc.wantName)
			}
			if strings.Join(args, ",") != strings.Join(tc.wantArgs, ",") {
				t.Errorf("args = %v want %v", args, tc.wantArgs)
			}
		})
	}
}

func TestOpenResource(t *testing.T) {
	io, _, _, errBuf := iostreams.Test()
	var opened string
	orig := BrowserRunner
	BrowserRunner = func(name string, args ...string) error { opened = args[len(args)-1]; return nil }
	defer func() { BrowserRunner = orig }()

	// With a link: opens it and notes the URL on stderr.
	if err := OpenResource(io, "deal", "D1", "https://crm/d/1"); err != nil {
		t.Fatal(err)
	}
	if opened != "https://crm/d/1" || !strings.Contains(errBuf.String(), "Opening") {
		t.Errorf("opened=%q stderr=%q", opened, errBuf.String())
	}

	// Without a link: uniform error, no browser launch.
	opened = ""
	if err := OpenResource(io, "account", "A1", ""); err == nil {
		t.Error("expected error for missing link")
	}
	if opened != "" {
		t.Error("should not launch the browser without a link")
	}
}

func TestOpenBrowserUsesRunner(t *testing.T) {
	var gotName string
	var gotArgs []string
	orig := BrowserRunner
	BrowserRunner = func(name string, args ...string) error {
		gotName, gotArgs = name, args
		return nil
	}
	defer func() { BrowserRunner = orig }()

	if err := OpenBrowser("https://app.modjo.ai"); err != nil {
		t.Fatal(err)
	}
	if gotName == "" || len(gotArgs) == 0 || gotArgs[len(gotArgs)-1] != "https://app.modjo.ai" {
		t.Errorf("runner got name=%q args=%v", gotName, gotArgs)
	}
}

func TestPromptSecretNonTTY(t *testing.T) {
	io, in, _, _ := iostreams.Test()
	in.WriteString("secret-value\n")
	got, err := PromptSecret(io, "key: ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret-value" {
		t.Errorf("got %q", got)
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"\n", true, true},
		{"\n", false, false},
		{"garbage\n", true, false},
	}
	for _, tc := range cases {
		io, in, _, _ := iostreams.Test()
		io.SetNeverPrompt(false)
		in.WriteString(tc.in)
		got, err := Confirm(io, "ok?", tc.def)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("Confirm(%q, def=%v) = %v want %v", tc.in, tc.def, got, tc.want)
		}
	}
}

func TestConfirmNoPromptReturnsDefault(t *testing.T) {
	io, _, _, _ := iostreams.Test() // neverPrompt = true
	got, err := Confirm(io, "ok?", true)
	if err != nil || !got {
		t.Errorf("got %v, %v want true, nil", got, err)
	}
}

func TestNormalizeDateFlag(t *testing.T) {
	f := &Factory{Clock: text.FixedClock(mustNow())}
	got, err := NormalizeDateFlag(f, "2026-05-01")
	if err != nil || got != "2026-05-01" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := NormalizeDateFlag(f, "05/01/2026"); err == nil {
		t.Fatal("expected usage error")
	} else {
		var ue *UsageError
		if !errors.As(err, &ue) {
			t.Errorf("want *UsageError, got %T", err)
		}
	}
}

func TestSaveConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f := &Factory{ConfigPath: dir + "/config.toml", Flags: &GlobalFlags{}}
	cfg := config.New()
	cfg.ActiveProfile = "x"
	if err := f.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir + "/config.toml")
	if err != nil || loaded.ActiveProfile != "x" {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestOSLookup(t *testing.T) {
	t.Setenv("MODJO_TEST_VAR", "v")
	if v, ok := OSLookup("MODJO_TEST_VAR"); !ok || v != "v" {
		t.Errorf("OSLookup = %q, %v", v, ok)
	}
	if _, ok := OSLookup("MODJO_DEFINITELY_UNSET_XYZ"); ok {
		t.Error("unset var reported present")
	}
}

func TestIgnoreEOF(t *testing.T) {
	if ignoreEOF(nil) != nil {
		t.Error("nil should stay nil")
	}
	if ignoreEOF(errors.New("EOF")) != nil {
		t.Error("EOF should be swallowed")
	}
	if ignoreEOF(errors.New("boom")) == nil {
		t.Error("other errors should pass through")
	}
}
