package root_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmd/root"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

// errorHarness runs commands against a server that always 500s, with a valid
// stored credential — so the API/MCP client error branches (the `if err != nil`
// after each call) execute.
func errorHarness(t *testing.T) func(args ...string) error {
	t.Helper()
	// 400 is non-retryable, so commands fail fast without backoff delays.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MODJO_BASE_URL", srv.URL)
	t.Setenv("MODJO_MCP_URL", srv.URL)

	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { return nil }
	t.Cleanup(func() { cmdutil.BrowserRunner = orig })

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t", Method: auth.MethodAPIKey})
	cfgPath := t.TempDir() + "/config.toml"

	return func(args ...string) error {
		io, _, out, errOut := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{MaxRetries: 1},
			Clock:      text.FixedClock(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)),
			ConfigPath: cfgPath,
			CredStore:  store,
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		return cmd.Execute()
	}
}

func TestCommandsSurfaceServerErrors(t *testing.T) {
	run := errorHarness(t)
	cmds := [][]string{
		{"calls", "list"},
		{"calls", "get", "1"},
		{"calls", "summary", "1"},
		{"calls", "transcript", "1"},
		{"calls", "open", "1"},
		{"calls", "export"},
		{"deals", "list"},
		{"deals", "get", "1"},
		{"deals", "open", "1"},
		{"accounts", "list", "--name", "x"},
		{"accounts", "get", "1"},
		{"accounts", "open", "1"},
		{"contacts", "list"},
		{"contacts", "get", "1"},
		{"emails", "list"},
		{"emails", "get", "1"},
		{"users", "list"},
		{"users", "get", "1"},
		{"users", "create", "--email", "x@y.com"},
		{"users", "delete", "1", "--yes"},
		{"teams", "list"},
		{"teams", "get", "1"},
		{"agents", "list"},
		{"agents", "get", "1"},
		{"ask", "deal", "1", "q"},
		{"ask", "call", "1", "q"},
		{"ask", "account", "1", "q"},
		{"mcp", "tools"},
		{"mcp", "call", "x"},
		{"api", "GET", "/deals"},
		{"api", "GET", "/deals", "--paginate"},
	}
	for _, args := range cmds {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := run(args...); err == nil {
				t.Errorf("%v should surface the server error", args)
			}
		})
	}
}

// brokenConfigHarness points commands at an unparseable config so the early
// f.Config()/f.APIClient() error branches execute.
func brokenConfigHarness(t *testing.T) func(args ...string) error {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/config.toml"
	if err := os.WriteFile(path, []byte("== not toml ]["), 0o600); err != nil {
		t.Fatal(err)
	}
	return func(args ...string) error {
		io, _, out, errOut := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{},
			Clock:      text.FixedClock(time.Now()),
			ConfigPath: path,
			CredStore:  auth.NewMemoryStore(),
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		return cmd.Execute()
	}
}

func TestCommandsSurfaceConfigErrors(t *testing.T) {
	run := brokenConfigHarness(t)
	cmds := [][]string{
		{"calls", "list"},
		{"calls", "get", "1"},
		{"calls", "summary", "1"},
		{"calls", "transcript", "1"},
		{"calls", "open", "1"},
		{"calls", "export"},
		{"deals", "list"},
		{"deals", "get", "1"},
		{"deals", "open", "1"},
		{"accounts", "list", "--name", "x"},
		{"accounts", "get", "1"},
		{"accounts", "open", "1"},
		{"contacts", "list"},
		{"contacts", "get", "1"},
		{"emails", "list"},
		{"emails", "get", "1"},
		{"users", "list"},
		{"users", "get", "1"},
		{"users", "create", "--email", "x@y.com"},
		{"teams", "list"},
		{"teams", "get", "1"},
		{"agents", "list"},
		{"agents", "get", "1"},
		{"config", "get", "output"},
		{"config", "set", "output", "json"},
		{"config", "list"},
		{"profiles", "list"},
		{"profiles", "use", "x"},
		{"auth", "switch", "x"},
		{"ask", "deal", "1", "q"},
		{"ask", "call", "1", "q"},
		{"ask", "account", "1", "q"},
		{"mcp", "tools"},
		{"mcp", "call", "x"},
		{"mcp", "serve"},
		{"api", "GET", "/x"},
		{"doctor"},
	}
	for _, args := range cmds {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := run(args...); err == nil {
				t.Errorf("%v should surface the config error", args)
			}
		})
	}
}
