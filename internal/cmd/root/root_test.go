package root_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmd/root"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

// newTestFactory wires a Factory with in-memory streams, a memory credential
// store seeded with a token, and a base transport pointing at srv.
func newTestFactory(t *testing.T, srv *httptest.Server) (*cmdutil.Factory, *strings.Builder) {
	t.Helper()
	// Path-less base URL so request paths are exactly "/deals", "/calls", etc.
	t.Setenv("MODJO_BASE_URL", "http://modjo.local")
	t.Setenv("MODJO_MCP_URL", "http://modjo.local/mcp")

	io, _, _, _ := iostreams.Test()
	out := &strings.Builder{}
	io.Out = out
	io.SetStdoutTTY(false)

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "test-token", Method: auth.MethodAPIKey})

	cfgPath := t.TempDir() + "/config.toml"
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		Clock:      text.FixedClock(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)),
		ConfigPath: cfgPath,
		CredStore:  store,
		Transport:  rewriteTransport{base: srv.URL},
	}
	return f, out
}

// rewriteTransport sends every request to the test server, preserving the path.
type rewriteTransport struct{ base string }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := rt.base + req.URL.Path
	if req.URL.RawQuery != "" {
		u += "?" + req.URL.RawQuery
	}
	nr, err := http.NewRequest(req.Method, u, req.Body)
	if err != nil {
		return nil, err
	}
	nr.Header = req.Header
	return http.DefaultTransport.RoundTrip(nr)
}

func runCmd(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestDealsListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deals" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header = %q", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"crmId":"D1","name":"Contoso","status":"Open","amount":42000}],"pagination":{"nextCursor":""}}`))
	}))
	defer srv.Close()

	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "deals", "list", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"crmId": "D1"`) {
		t.Errorf("output missing deal:\n%s", out.String())
	}
}

func TestDealsListStatusAliasMapped(t *testing.T) {
	var sawStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawStatus = r.URL.Query().Get("status")
		_, _ = w.Write([]byte(`{"values":[],"pagination":{}}`))
	}))
	defer srv.Close()

	f, _ := newTestFactory(t, srv)
	if err := runCmd(t, f, "deals", "list", "--status", "won", "--json"); err != nil {
		t.Fatal(err)
	}
	if sawStatus != "Closed won" {
		t.Errorf("status alias not mapped: query had status=%q", sawStatus)
	}
}

func TestCallsListCSVColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":[{"id":74969,"title":"Discovery","startDate":"2026-05-20","summary":"Talked pricing"}],"pagination":{}}`))
	}))
	defer srv.Close()

	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "calls", "list", "-o", "csv", "--columns", "id,title"); err != nil {
		t.Fatal(err)
	}
	want := "ID,TITLE\n74969,Discovery\n"
	if out.String() != want {
		t.Errorf("csv output:\n%q\nwant\n%q", out.String(), want)
	}
}

func TestUserDeleteDryRun(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	defer srv.Close()

	f, _ := newTestFactory(t, srv)
	if err := runCmd(t, f, "users", "delete", "42", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("dry-run should not call the API")
	}
}

func TestVersionCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	f, out := newTestFactory(t, srv)
	if err := runCmd(t, f, "version"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "modjo ") {
		t.Errorf("version output = %q", out.String())
	}
}
