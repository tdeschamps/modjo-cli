package info

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func newFactory(t *testing.T, tty bool) (*cmdutil.Factory, *strings.Builder) {
	t.Helper()
	io, _, _, _ := iostreams.Test()
	out := &strings.Builder{}
	io.Out = out
	io.SetStdoutTTY(tty)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "mjo_live_abcd1234", Method: auth.MethodAPIKey, Workspace: "acme-eu"})
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/config.toml",
		CredStore:  store,
	}
	return f, out
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdInfo(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestInfoPipedIsPlainJSON(t *testing.T) {
	f, out := newFactory(t, false) // non-TTY → JSON
	if err := run(t, f); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	// Must be machine-readable JSON, with no logo or ANSI escapes.
	if !strings.Contains(s, `"version"`) || !strings.Contains(s, `"authenticated": true`) {
		t.Errorf("expected JSON fields:\n%s", s)
	}
	if strings.Contains(s, "█") || strings.Contains(s, "\033") {
		t.Errorf("piped output must not contain the logo or color: %q", s)
	}
	// The raw token must never appear — only a fingerprint.
	if strings.Contains(s, "mjo_live_abcd1234") {
		t.Errorf("piped info leaked the token: %s", s)
	}
}

func TestInfoHumanBanner(t *testing.T) {
	f, out := newFactory(t, true)
	f.Flags.Output = "table" // TTY + table → human banner
	if err := run(t, f); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"█", "Version", "Profile", "acme-eu", "Auth", "Docs"} {
		if !strings.Contains(s, want) {
			t.Errorf("human banner missing %q:\n%s", want, s)
		}
	}
}

func TestInfoUnauthenticatedHuman(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	out := &strings.Builder{}
	io.Out = out
	io.SetStdoutTTY(true)
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{Output: "table"},
		ConfigPath: t.TempDir() + "/config.toml",
		CredStore:  auth.NewMemoryStore(), // empty → not logged in
	}
	if err := run(t, f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not logged in") || !strings.Contains(out.String(), "auth login") {
		t.Errorf("unauthenticated banner should prompt login:\n%s", out.String())
	}
}

func TestInfoCheckProbes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/users") {
			_, _ = w.Write([]byte(`{"data":[{"id":1}],"pagination":{}}`))
			return
		}
		// MCP endpoint
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL+"/v2")
	t.Setenv("MODJO_MCP_URL", srv.URL+"/mcp")

	f, out := newFactory(t, false)
	if err := run(t, f, "--check"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, `"restReachable": true`) || !strings.Contains(s, `"mcpReachable": true`) {
		t.Errorf("--check should report reachability:\n%s", s)
	}
}
