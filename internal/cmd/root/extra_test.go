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

func TestPresentationFlags(t *testing.T) {
	run, _ := harness(t)
	for _, args := range [][]string{
		{"version", "--color", "always"},
		{"version", "--color", "never"},
		{"version", "--no-color"},
		{"version", "--quiet"},
	} {
		if _, _, err := run(args...); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestMCPServeHTTPUnsupported(t *testing.T) {
	run, _ := harness(t)
	if _, _, err := run("mcp", "serve", "--transport", "http"); err == nil {
		t.Error("http transport should be unsupported")
	}
}

func TestMCPServeStdioEOF(t *testing.T) {
	// With empty stdin the stdio server reads EOF immediately and exits cleanly.
	run, _ := harness(t)
	if _, _, err := run("mcp", "serve"); err != nil {
		t.Errorf("mcp serve stdio: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("--version")
	if err != nil || !strings.Contains(out, "modjo ") {
		t.Fatalf("--version: %v %s", err, out)
	}
}

func TestNoArgsShowsHelp(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run()
	if err != nil || !strings.Contains(out, "Core commands") {
		t.Fatalf("no args should show help: %v %s", err, out)
	}
}

func TestHelpBoldStandardTitlesWithColorEnabled(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("calls", "--help", "--color", "always")
	if err != nil {
		t.Fatalf("calls --help --color always: %v", err)
	}
	for _, want := range []string{
		"\x1b[1mUsage:\x1b[0m",
		"\x1b[1mAvailable Commands:\x1b[0m",
		"\x1b[1mFlags:\x1b[0m",
		"\x1b[1mGlobal Flags:\x1b[0m",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected bold heading %q in help output:\n%s", want, out)
		}
	}
}

func TestHelpTitlesRemainPlainWhenColorDisabled(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("calls", "--help", "--color", "never")
	if err != nil {
		t.Fatalf("calls --help --color never: %v", err)
	}
	for _, want := range []string{"Usage:", "Available Commands:", "Flags:", "Global Flags:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected plain heading %q in help output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[1m") {
		t.Fatalf("did not expect ANSI bold sequences with color disabled:\n%s", out)
	}
}

func TestCompletionBadShell(t *testing.T) {
	run, _ := harness(t)
	if _, _, err := run("completion", "tcsh"); err == nil {
		t.Error("unsupported shell should error")
	}
}

func TestListBadDateFlags(t *testing.T) {
	run, _ := harness(t)
	for _, args := range [][]string{
		{"calls", "list", "--since", "bad/date"},
		{"calls", "list", "--until", "xyz"},
		{"calls", "export", "--since", "13/13/13"},
	} {
		if _, _, err := run(args...); err == nil {
			t.Errorf("%v should reject the bad date", args)
		}
	}
}

func TestDealsListEmptyAmount(t *testing.T) {
	// fmtAmount returns "" when amount is zero — render a zero-amount deal.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"crmId":"D9","name":"Free","status":"Open"}],"pagination":{}}`))
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	io, _, out, _ := iostreams.Test()
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"deals", "list", "-o", "table"})
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Free") {
		t.Errorf("out: %s", out.String())
	}
}

// noLinkHarness serves objects without a crmLink so `open` hits its error path.
func noLinkHarness(t *testing.T) func(args ...string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/calls/"):
			_, _ = w.Write([]byte(`{"id":1,"name":"t"}`))
		case strings.Contains(r.URL.Path, "/deals/"):
			_, _ = w.Write([]byte(`{"crmId":"D1","name":"n"}`))
		case strings.Contains(r.URL.Path, "/accounts/"):
			_, _ = w.Write([]byte(`{"crmId":"A1","name":"n"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	cfg := t.TempDir() + "/c.toml"
	return func(args ...string) error {
		io, _, out, errOut := iostreams.Test()
		f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, Clock: text.FixedClock(time.Now()), ConfigPath: cfg, CredStore: store}
		cmd := root.NewCmdRoot(f)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		return cmd.Execute()
	}
}

func TestOpenWithoutCRMLink(t *testing.T) {
	run := noLinkHarness(t)
	for _, args := range [][]string{
		{"accounts", "open", "A1"},
	} {
		if err := run(args...); err == nil {
			t.Errorf("%v should error without a crmLink", args)
		}
	}
}

func TestTranscriptNonJSONFormatsAreMachineRendered(t *testing.T) {
	run, _ := harness(t)
	// CSV must render the transcript blocks as CSV, not the human text dump.
	out, _, err := run("calls", "transcript", "74969", "-o", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "START,END,SPEAKER,CONTENT") {
		t.Errorf("csv transcript should have a header row, got:\n%s", out)
	}
	// YAML likewise.
	out, _, err = run("calls", "transcript", "74969", "-o", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "speakerName") && !strings.Contains(out, "Alice") {
		t.Errorf("yaml transcript should render structured blocks, got:\n%s", out)
	}
}

func TestJQPromotesToJSONInTableContext(t *testing.T) {
	srv := fullStub(t)
	t.Setenv("MODJO_BASE_URL", srv.URL+"/v2")
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true) // TTY → default format would be table
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		Clock:      text.FixedClock(time.Now()),
		ConfigPath: t.TempDir() + "/c.toml",
		CredStore:  store,
	}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"deals", "list", "--jq", ".[].crmId"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// --jq must take effect even though the format would default to table.
	if strings.TrimSpace(out.String()) != "D1" {
		t.Errorf("--jq should filter (promoted to JSON), got:\n%s", out.String())
	}
}

func TestSummaryMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"name":"t"}`)) // no summary
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	io, _, out, errOut := iostreams.Test()
	io.SetStdoutTTY(true) // interactive → the "No summary" notice path runs
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"calls", "summary", "1"})
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "No summary") {
		t.Errorf("expected no-summary notice, got %s", errOut.String())
	}
}

func TestDeleteConfirmDecline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("delete should not call the API when declined")
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})

	io, in, _, _ := iostreams.Test()
	io.SetNeverPrompt(false)
	in.WriteString("n\n") // decline
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"users", "delete", "7"})
	cmd.SetOut(io.Out)
	cmd.SetErr(io.ErrOut)
	// Declining aborts with a silent exit-0 error.
	_ = cmd.Execute()
}

func TestCallSummaryEmpty(t *testing.T) {
	// An interactive `calls summary` with no summaries hits the "No summary
	// available" branch instead of rendering rows.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})

	io, _, _, errBuf := iostreams.Test()
	io.SetStdoutTTY(true) // force the interactive (non-machine) path
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"calls", "summary", "42"})
	cmd.SetOut(io.Out)
	cmd.SetErr(errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("summary (empty): %v", err)
	}
	if !strings.Contains(errBuf.String(), "No summary available") {
		t.Errorf("expected empty-summary notice, got: %s", errBuf.String())
	}
}

func TestWebhookDeleteConfirmProceed(t *testing.T) {
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})

	io, in, _, _ := iostreams.Test()
	io.SetNeverPrompt(false)
	in.WriteString("y\n") // confirm
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"webhooks", "delete", "wh-1"})
	cmd.SetOut(io.Out)
	cmd.SetErr(io.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete (confirmed): %v", err)
	}
	if !deleted {
		t.Error("confirmed delete should call the API")
	}
}

func TestDoctorNoCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":"me"}`))
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	t.Setenv("MODJO_MCP_URL", srv.URL)
	io, _, out, _ := iostreams.Test()
	// Empty store → no credential branch in doctor.
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: auth.NewMemoryStore()}
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs([]string{"doctor"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "credential stored") {
		t.Errorf("doctor output: %s", out.String())
	}
}
