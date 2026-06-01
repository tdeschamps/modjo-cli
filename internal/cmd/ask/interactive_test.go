package ask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

// interactiveStub serves the accounts search + the MCP ask used by the picker.
func interactiveStub(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/accounts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"crmId":"A1","name":"Contoso"}],"pagination":{}}`))
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		res := `{}`
		if req.Method == "tools/call" {
			res = `{"content":[{"type":"text","text":"The relationship is healthy."}]}`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":` + res + `}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestAskInteractiveAccountFlow(t *testing.T) {
	url := interactiveStub(t)
	t.Setenv("MODJO_BASE_URL", url+"/v2")
	t.Setenv("MODJO_MCP_URL", url+"/mcp")

	io, in, out, _ := iostreams.Test()
	io.SetNeverPrompt(false) // allow prompting
	// 3 = account, then search term, then the question.
	in.WriteString("3\nContoso\nWhat is the relationship like?\n")

	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{JSON: true},
		ConfigPath: t.TempDir() + "/c.toml",
		CredStore:  store,
	}

	cmd := NewCmdAsk(f)
	cmd.SetArgs(nil) // no subcommand → interactive
	cmd.SetOut(out)
	cmd.SetErr(io.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("interactive ask: %v", err)
	}
	if !strings.Contains(out.String(), "relationship is healthy") {
		t.Errorf("expected the AI answer, got: %s", out.String())
	}
}

func TestAskInteractiveNonTTYShowsHelp(t *testing.T) {
	io, _, out, _ := iostreams.Test() // neverPrompt = true
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml"}
	cmd := NewCmdAsk(f)
	cmd.SetArgs(nil)
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "natural-language") {
		t.Errorf("non-TTY ask should show help, got: %s", out.String())
	}
}

// runInteractiveWith drives the picker with the given stdin and base URLs.
func runInteractiveWith(t *testing.T, baseURL, mcpURL, stdin string) (string, error) {
	t.Helper()
	t.Setenv("MODJO_BASE_URL", baseURL)
	t.Setenv("MODJO_MCP_URL", mcpURL)
	io, in, out, _ := iostreams.Test()
	io.SetNeverPrompt(false)
	in.WriteString(stdin)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{JSON: true}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := NewCmdAsk(f)
	cmd.SetArgs(nil)
	cmd.SetOut(out)
	cmd.SetErr(io.ErrOut)
	err := cmd.Execute()
	return out.String(), err
}

func TestAskInteractiveCallByID(t *testing.T) {
	url := interactiveStub(t)
	// 1 = call, then the call ID, then the question.
	out, err := runInteractiveWith(t, url+"/v2", url+"/mcp", "1\n74969\nWhat objections?\n")
	if err != nil {
		t.Fatalf("call flow: %v", err)
	}
	if !strings.Contains(out, "relationship is healthy") {
		t.Errorf("expected answer, got %s", out)
	}
}

func TestAskInteractiveNoAccountMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/accounts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := runInteractiveWith(t, srv.URL+"/v2", srv.URL+"/mcp", "3\nNoSuch\n"); err == nil {
		t.Error("expected an error when no accounts match")
	}
}

func TestAskInteractiveBadSelection(t *testing.T) {
	url := interactiveStub(t)
	if _, err := runInteractiveWith(t, url+"/v2", url+"/mcp", "9\n"); err == nil {
		t.Error("expected an error for an out-of-range entity choice")
	}
}
