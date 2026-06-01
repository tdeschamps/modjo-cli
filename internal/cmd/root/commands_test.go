package root_test

import (
	"encoding/json"
	"io"
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

// fullStub serves the slice of the REST API + MCP endpoint the command tests
// exercise. Routing is by path so a single server backs both REST and MCP.
func fullStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	list := func(values string) string {
		return `{"data":[` + values + `],"pagination":{}}`
	}
	mux.HandleFunc("/v2/calls", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"id":74969,"name":"Discovery","date":"2026-05-20"}`)))
	})
	// Call sub-resources (transcript, summaries) return their own {data:[...]}
	// envelopes; the bare GET /calls/{id} returns the CallExpanded object.
	mux.HandleFunc("/v2/calls/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transcript"):
			_, _ = w.Write([]byte(list(`{"startTime":12,"endTime":20,"speaker":{"name":"Alice"},"content":"Hello there"}`)))
		case strings.HasSuffix(r.URL.Path, "/summaries"):
			_, _ = w.Write([]byte(list(`{"uuid":"s1","templateTitle":"Recap","templateLength":"short","answer":"Pricing recap"}`)))
		default:
			_, _ = w.Write([]byte(`{"id":74969,"name":"Discovery","date":"2026-05-20","crmLink":"https://crm/c/1"}`))
		}
	})
	mux.HandleFunc("/v2/deals", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"crmId":"D1","name":"Contoso","status":"Open","amount":42000,"currency":"EUR","crmLink":"https://crm/d/1"}`)))
	})
	mux.HandleFunc("/v2/deals/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"crmId":"D1","name":"Contoso","status":"Open","amount":42000,"crmLink":"https://crm/d/1"}`))
	})
	mux.HandleFunc("/v2/accounts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"crmId":"A1","name":"Contoso","domain":"contoso.com"}`)))
	})
	mux.HandleFunc("/v2/accounts/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"crmId":"A1","name":"Contoso","crmLink":"https://crm/a/1"}`))
	})
	mux.HandleFunc("/v2/contacts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"crmPersonId":"P1","name":"Jean Martin","email":"jean@contoso.com"}`)))
	})
	mux.HandleFunc("/v2/contacts/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"crmPersonId":"P1","name":"Jean Martin"}`))
	})
	mux.HandleFunc("/v2/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":99,"email":"new@acme.com","role":"rep"}`))
			return
		}
		_, _ = w.Write([]byte(list(`{"id":1,"email":"me@acme.com","name":"Me","role":"admin"}`)))
	})
	mux.HandleFunc("/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"email":"me@acme.com","name":"Me","role":"admin"}`))
	})
	mux.HandleFunc("/v2/teams", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"id":3,"name":"EMEA"}`)))
	})
	mux.HandleFunc("/v2/teams/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":3,"name":"EMEA"}`))
	})
	mux.HandleFunc("/v2/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"id":11,"name":"Pricing","color":"#ff0000"}`)))
	})
	mux.HandleFunc("/v2/topics", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list(`{"id":22,"name":"Competition","slug":"competition","saidBy":"contact"}`)))
	})
	mux.HandleFunc("/v2/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"uuid":"wh-1","name":"My hook","url":"https://hooks/x","events":["call_summarized"]}`))
			return
		}
		_, _ = w.Write([]byte(list(`{"uuid":"wh-1","name":"My hook","url":"https://hooks/x","events":["call_summarized"]}`)))
	})
	mux.HandleFunc("/v2/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"uuid":"wh-1","name":"My hook","url":"https://hooks/x","events":["call_summarized"]}`))
	})
	// MCP JSON-RPC endpoint.
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		write := func(result string) {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":` + result + `}`))
		}
		switch req.Method {
		case "initialize":
			write(`{"protocolVersion":"2025-06-18"}`)
		case "tools/list":
			write(`{"tools":[{"name":"ask_anything_on_deal","description":"Ask about a deal"}]}`)
		case "tools/call":
			write(`{"content":[{"type":"text","text":"The main risk is pricing."}]}`)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func harness(t *testing.T) (func(args ...string) (string, string, error), *iostreams.IOStreams) {
	t.Helper()
	srv := fullStub(t)
	t.Setenv("MODJO_BASE_URL", srv.URL+"/v2")
	t.Setenv("MODJO_MCP_URL", srv.URL+"/mcp")

	// Stub the browser so `open` commands don't launch anything.
	orig := cmdutil.BrowserRunner
	cmdutil.BrowserRunner = func(string, ...string) error { return nil }
	t.Cleanup(func() { cmdutil.BrowserRunner = orig })

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "test-token", Method: auth.MethodAPIKey, Workspace: "acme-eu"})
	cfgPath := t.TempDir() + "/config.toml"

	run := func(args ...string) (string, string, error) {
		io, _, outBuf, errBuf := iostreams.Test()
		f := &cmdutil.Factory{
			IOStreams:  io,
			Flags:      &cmdutil.GlobalFlags{},
			Clock:      text.FixedClock(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)),
			ConfigPath: cfgPath,
			CredStore:  store,
		}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)
		err := cmd.Execute()
		return outBuf.String(), errBuf.String(), err
	}
	return run, nil
}

func TestAllListCommands(t *testing.T) {
	run, _ := harness(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"calls", "list", "--json"}, "Discovery"},
		{[]string{"calls", "get", "74969", "--json"}, "Discovery"},
		{[]string{"calls", "summary", "74969"}, "Pricing recap"},
		{[]string{"calls", "transcript", "74969", "-o", "table"}, "Alice"},
		{[]string{"calls", "transcript", "74969", "--timestamps", "--speakers", "-o", "table"}, "00:12"},
		{[]string{"calls", "transcript", "74969", "--json"}, "Hello there"},
		{[]string{"calls", "export", "--since", "30d"}, "Discovery"},
		{[]string{"deals", "list", "--json"}, "Contoso"},
		{[]string{"deals", "get", "D1", "--json"}, "Contoso"},
		{[]string{"deals", "open", "D1"}, ""},
		{[]string{"accounts", "list", "--name", "Contoso", "--json"}, "Contoso"},
		{[]string{"accounts", "get", "A1", "--json"}, "Contoso"},
		{[]string{"accounts", "open", "A1"}, ""},
		{[]string{"contacts", "list", "--json"}, "Jean Martin"},
		{[]string{"contacts", "get", "P1", "--json"}, "Jean Martin"},
		{[]string{"users", "list", "--json"}, "me@acme.com"},
		{[]string{"users", "get", "1", "--json"}, "admin"},
		{[]string{"teams", "list", "--json"}, "EMEA"},
		{[]string{"teams", "get", "3", "--json"}, "EMEA"},
		{[]string{"tags", "list", "--json"}, "Pricing"},
		{[]string{"topics", "list", "--json"}, "Competition"},
		{[]string{"webhooks", "list", "--json"}, "My hook"},
		{[]string{"webhooks", "get", "wh-1", "--json"}, "My hook"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			out, errOut, err := run(tc.args...)
			if err != nil {
				t.Fatalf("err: %v (stderr: %s)", err, errOut)
			}
			if tc.want != "" && !strings.Contains(out+errOut, tc.want) {
				t.Errorf("output missing %q:\nout=%s\nerr=%s", tc.want, out, errOut)
			}
		})
	}
}

// TestTableOutputAllResources renders every resource as a table/csv so the
// per-column Extract closures (and helpers like fmtAmount/truncate) execute.
func TestTableOutputAllResources(t *testing.T) {
	run, _ := harness(t)
	cases := [][]string{
		{"calls", "list", "-o", "table"},
		{"calls", "get", "74969", "-o", "table"},
		{"deals", "list", "-o", "table"},
		{"deals", "get", "D1", "-o", "csv"},
		{"accounts", "list", "--name", "Contoso", "-o", "table"},
		{"accounts", "get", "A1", "-o", "csv"},
		{"contacts", "list", "-o", "table"},
		{"contacts", "get", "P1", "-o", "csv"},
		{"users", "list", "-o", "table"},
		{"users", "get", "1", "-o", "csv"},
		{"teams", "list", "-o", "table"},
		{"teams", "get", "3", "-o", "csv"},
		{"tags", "list", "-o", "table"},
		{"topics", "list", "-o", "table"},
		{"webhooks", "list", "-o", "table"},
		{"webhooks", "get", "wh-1", "-o", "csv"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if _, errOut, err := run(args...); err != nil {
				t.Fatalf("%v: %v (%s)", args, err, errOut)
			}
		})
	}
}

func TestColumnsAndYAML(t *testing.T) {
	run, _ := harness(t)
	if out, _, err := run("deals", "list", "-o", "yaml"); err != nil || !strings.Contains(out, "Contoso") {
		t.Fatalf("yaml: %v %s", err, out)
	}
	if out, _, err := run("deals", "list", "-o", "csv", "--columns", "name,amount"); err != nil || !strings.Contains(out, "NAME,AMOUNT") {
		t.Fatalf("columns: %v %s", err, out)
	}
	if out, _, err := run("deals", "list", "--json", "--jq", ".[].name"); err != nil || !strings.Contains(out, "Contoso") {
		t.Fatalf("jq: %v %s", err, out)
	}
}

// TestBadOutputFormatAcrossCommands exercises the Printer/RenderSlice error
// branch in every renderer command (client succeeds, formatting fails).
func TestBadOutputFormatAcrossCommands(t *testing.T) {
	run, _ := harness(t)
	cmds := [][]string{
		{"calls", "list", "-o", "bogus"},
		{"calls", "get", "74969", "-o", "bogus"},
		{"deals", "list", "-o", "bogus"},
		{"deals", "get", "D1", "-o", "bogus"},
		{"accounts", "list", "--name", "x", "-o", "bogus"},
		{"accounts", "get", "A1", "-o", "bogus"},
		{"contacts", "list", "-o", "bogus"},
		{"contacts", "get", "P1", "-o", "bogus"},
		{"users", "list", "-o", "bogus"},
		{"users", "get", "1", "-o", "bogus"},
		{"teams", "list", "-o", "bogus"},
		{"teams", "get", "3", "-o", "bogus"},
		{"tags", "list", "-o", "bogus"},
		{"topics", "list", "-o", "bogus"},
		{"webhooks", "list", "-o", "bogus"},
		{"webhooks", "get", "wh-1", "-o", "bogus"},
		{"calls", "transcript", "74969", "-o", "bogus"},
	}
	for _, args := range cmds {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if _, _, err := run(args...); err == nil {
				t.Errorf("%v should reject the bad format", args)
			}
		})
	}
}

func TestAskCommands(t *testing.T) {
	run, _ := harness(t)
	for _, args := range [][]string{
		{"ask", "deal", "D1", "What are the risks?"},
		{"ask", "deal", "D1", "What are the risks?", "--json"},
		{"ask", "call", "74969", "objections?"},
		{"ask", "account", "A1", "summary?"},
	} {
		out, errOut, err := run(args...)
		if err != nil {
			t.Fatalf("%v: %v (%s)", args, err, errOut)
		}
		if !strings.Contains(out+errOut, "pricing") {
			t.Errorf("%v: missing answer: %s %s", args, out, errOut)
		}
	}
}

func TestMCPCommands(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("mcp", "tools", "--json")
	if err != nil || !strings.Contains(out, "ask_anything_on_deal") {
		t.Fatalf("mcp tools: %v %s", err, out)
	}
	out, _, err = run("mcp", "tools")
	if err != nil || !strings.Contains(out, "ask_anything_on_deal") {
		t.Fatalf("mcp tools table: %v %s", err, out)
	}
	out, _, err = run("mcp", "call", "ask_anything_on_deal", "--args", `{"crmId":"D1"}`)
	if err != nil || !strings.Contains(out, "pricing") {
		t.Fatalf("mcp call: %v %s", err, out)
	}
	out, _, _ = run("mcp", "config", "--client", "claude-desktop")
	if !strings.Contains(out, "mcpServers") {
		t.Errorf("mcp config: %s", out)
	}
	for _, c := range []string{"cursor", "codex", ""} {
		args := []string{"mcp", "config"}
		if c != "" {
			args = append(args, "--client", c)
		}
		if _, _, err := run(args...); err != nil {
			t.Errorf("mcp config %q: %v", c, err)
		}
	}
}

func TestAPICommand(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("api", "GET", "/deals")
	if err != nil || !strings.Contains(out, "Contoso") {
		t.Fatalf("api GET: %v %s", err, out)
	}
	// shorthand path → GET
	if _, _, err := run("api", "/deals"); err != nil {
		t.Fatalf("api shorthand: %v", err)
	}
	// paginate
	if out, _, err := run("api", "GET", "/deals", "--paginate"); err != nil || !strings.Contains(out, "Contoso") {
		t.Fatalf("api paginate: %v %s", err, out)
	}
	// POST with field
	if _, _, err := run("api", "POST", "/users", "--field", "email=new@acme.com"); err != nil {
		t.Fatalf("api POST field: %v", err)
	}
	// bad param
	if _, _, err := run("api", "GET", "/deals", "--param", "novalue"); err == nil {
		t.Error("expected bad --param error")
	}
}

func TestAPIPaginateSendsBody(t *testing.T) {
	var gotBodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBodies = append(gotBodies, string(b))
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"data":[{"id":1}],"pagination":{"page":1,"size":1,"total":2}}`))
		} else {
			_, _ = w.Write([]byte(`{"data":[{"id":2}],"pagination":{"page":2,"size":1,"total":2}}`))
		}
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	io2, _, out, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io2, Flags: &cmdutil.GlobalFlags{}, Clock: text.FixedClock(time.Now()), ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"api", "POST", "/search", "--paginate", "--field", "q=foo"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(gotBodies) != 2 {
		t.Fatalf("expected 2 page requests, got %d", len(gotBodies))
	}
	for i, b := range gotBodies {
		if !strings.Contains(b, `"q":"foo"`) {
			t.Errorf("page %d request body missing payload: %q", i, b)
		}
	}
}

func TestConfigAndProfileCommands(t *testing.T) {
	run, _ := harness(t)
	if _, _, err := run("config", "set", "output", "json"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	if out, _, err := run("config", "get", "output"); err != nil || !strings.Contains(out, "json") {
		t.Fatalf("config get: %v %s", err, out)
	}
	if out, _, err := run("config", "list"); err != nil || !strings.Contains(out, "base_url") {
		t.Fatalf("config list: %v %s", err, out)
	}
	if _, _, err := run("config", "set", "bogus", "x"); err == nil {
		t.Error("expected unknown-key error")
	}
	if _, _, err := run("config", "set", "default_limit", "notint"); err == nil {
		t.Error("expected int parse error")
	}
	if _, _, err := run("profiles", "use", "work"); err != nil {
		t.Fatalf("profiles use: %v", err)
	}
	if out, _, err := run("profiles", "list"); err != nil || !strings.Contains(out, "work") {
		t.Fatalf("profiles list: %v %s", err, out)
	}
}

func TestAuthCommands(t *testing.T) {
	run, _ := harness(t)
	if out, _, err := run("auth", "status"); err != nil || !strings.Contains(out, "Method") {
		t.Fatalf("auth status: %v %s", err, out)
	}
	if _, _, err := run("auth", "token"); err == nil {
		t.Error("auth token without --confirm should fail")
	}
	if out, _, err := run("auth", "token", "--confirm"); err != nil || !strings.Contains(out, "test-token") {
		t.Fatalf("auth token --confirm: %v %s", err, out)
	}
	if _, _, err := run("auth", "refresh"); err == nil {
		t.Error("refresh on api_key profile should error")
	}
	if _, errOut, err := run("auth", "logout"); err != nil || !strings.Contains(errOut, "Logged out") {
		t.Fatalf("auth logout: %v %s", err, errOut)
	}
	// logout again is a no-op (already removed).
	if _, errOut, err := run("auth", "logout"); err != nil || !strings.Contains(errOut, "No credentials") {
		t.Fatalf("auth logout (repeat): %v %s", err, errOut)
	}
	if _, _, err := run("auth", "switch", "other"); err != nil {
		t.Fatalf("auth switch: %v", err)
	}
}

func TestMiscCommands(t *testing.T) {
	run, _ := harness(t)
	for _, args := range [][]string{
		{"version"},
		{"doctor"},
		{"docs"},
		{"docs", "calls"},
		{"docs", "--web"},
		{"update"},
		{"completion", "bash"},
		{"completion", "zsh"},
		{"completion", "fish"},
		{"completion", "powershell"},
	} {
		if _, errOut, err := run(args...); err != nil {
			t.Errorf("%v: %v (%s)", args, err, errOut)
		}
	}
}

func TestUserWrites(t *testing.T) {
	run, _ := harness(t)
	if _, _, err := run("users", "create", "--first-name", "New", "--last-name", "Rep", "--email", "new@acme.com", "--role", "rep"); err != nil {
		t.Fatalf("users create: %v", err)
	}
	if _, errOut, err := run("users", "create", "--first-name", "X", "--last-name", "Y", "--email", "x@y.com", "--dry-run"); err != nil || !strings.Contains(errOut, "dry-run") {
		t.Fatalf("users create dry-run: %v %s", err, errOut)
	}
	if _, _, err := run("users", "create", "--email", "x@y.com"); err == nil {
		t.Error("users create without --first-name/--last-name should fail")
	}
	if _, errOut, err := run("users", "delete", "5", "--yes"); err != nil || !strings.Contains(errOut, "Deleted") {
		t.Fatalf("users delete --yes: %v %s", err, errOut)
	}
	if _, errOut, err := run("users", "delete", "5", "--dry-run"); err != nil || !strings.Contains(errOut, "dry-run") {
		t.Fatalf("users delete dry-run: %v %s", err, errOut)
	}
}
