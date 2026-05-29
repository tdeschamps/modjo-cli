package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func meServer(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`{"email":"me@acme.com"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func authFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *iostreams.IOStreams) {
	t.Helper()
	t.Setenv("MODJO_BASE_URL", baseURL)
	io, _, _, _ := iostreams.Test()
	store := auth.NewMemoryStore()
	return &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
		CredStore:  store,
	}, io
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdAuth(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestLoginWithToken(t *testing.T) {
	f, io := authFactory(t, meServer(t, 200))
	// Provide the token on stdin.
	writeStdin(io, "mjo_live_token_abcd1234\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatalf("login --with-token: %v", err)
	}
	store, _ := f.CredentialStore()
	cred, err := store.Get("default")
	if err != nil || cred.Token != "mjo_live_token_abcd1234" {
		t.Errorf("credential not stored: %+v %v", cred, err)
	}
}

func TestLoginInvalidKey(t *testing.T) {
	f, io := authFactory(t, meServer(t, 401))
	writeStdin(io, "badkey\n")
	if err := run(t, f, "login", "--with-token"); err == nil {
		t.Error("invalid key should fail validation")
	}
}

func TestLoginEmptyKey(t *testing.T) {
	f, io := authFactory(t, meServer(t, 200))
	writeStdin(io, "\n")
	if err := run(t, f, "login", "--with-token"); err == nil {
		t.Error("empty key should be a usage error")
	}
}

func TestLoginOAuthUnavailable(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	if err := run(t, f, "login", "--oauth"); err == nil {
		t.Error("oauth should be unavailable")
	}
}

func TestLoginNoPrompt(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	// neverPrompt is true for Test() streams → interactive paste path errors.
	if err := run(t, f, "login"); err == nil {
		t.Error("non-interactive login without --with-token should error")
	}
}

func TestStatusLoggedOut(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	if err := run(t, f, "status"); err == nil {
		t.Error("status should fail when not logged in")
	}
}

func TestRefreshOAuthProfile(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	store, _ := f.CredentialStore()
	_ = store.Set("default", auth.Credential{Token: "t", Method: auth.MethodOAuth})
	if err := run(t, f, "refresh"); err == nil {
		t.Error("oauth refresh is not yet available → error")
	}
}

func TestRefreshNoCredential(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	if err := run(t, f, "refresh"); err == nil {
		t.Error("refresh without credential should error")
	}
}

func TestMustResolve(t *testing.T) {
	f, _ := authFactory(t, "http://example/v2")
	if got := mustResolve(f, "base_url"); got != "http://example/v2" {
		t.Errorf("mustResolve = %q", got)
	}
}

func TestMustResolveBadConfigFallsBack(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	dir := t.TempDir()
	path := dir + "/c.toml"
	_ = os.WriteFile(path, []byte("== bad ]["), 0o600)
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: path}
	// Resolver errors on the broken config; mustResolve falls back to default.
	if got := mustResolve(f, "base_url"); got != "https://api.modjo.ai/v2" {
		t.Errorf("fallback = %q", got)
	}
}

func TestLoginWebOpensBrowser(t *testing.T) {
	opened := false
	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { opened = true; return nil }
	defer func() { cmdutil.BrowserRunner = orig }()

	f, _ := authFactory(t, meServer(t, 200))
	// --web opens the browser, then (non-interactively) fails to prompt.
	if err := run(t, f, "login", "--web"); err == nil {
		t.Error("non-interactive --web login should still fail to prompt")
	}
	if !opened {
		t.Error("--web should open the browser")
	}
}

func TestLoginAPIKeyFlag(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	// --api-key forces the paste flow which can't prompt non-interactively.
	if err := run(t, f, "login", "--api-key"); err == nil {
		t.Error("--api-key without a TTY should error")
	}
}

func TestLoginInteractivePaste(t *testing.T) {
	f, io := authFactory(t, meServer(t, 200))
	io.SetNeverPrompt(false)
	writeStdin(io, "pasted-key-value\n")
	if err := run(t, f, "login"); err != nil {
		t.Fatalf("interactive paste login: %v", err)
	}
	store, _ := f.CredentialStore()
	if cred, _ := store.Get("default"); cred.Token != "pasted-key-value" {
		t.Errorf("paste flow stored %q", cred.Token)
	}
}

func TestStatusShowsScopesAndExpiry(t *testing.T) {
	f, _ := authFactory(t, meServer(t, 200))
	store, _ := f.CredentialStore()
	_ = store.Set("default", auth.Credential{
		Token:     "mjo_live_abcd1234",
		Method:    auth.MethodOAuth,
		Workspace: "acme",
		Scopes:    []string{"calls:read"},
		Expiry:    time.Now().Add(time.Hour),
	})
	if err := run(t, f, "status"); err != nil {
		t.Fatalf("status: %v", err)
	}
}

// writeStdin replaces the stream's stdin with a reader over s.
func writeStdin(io *iostreams.IOStreams, s string) {
	io.In = strings.NewReader(s)
}
