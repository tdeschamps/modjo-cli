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

func TestCompletionBadShell(t *testing.T) {
	run, _ := harness(t)
	if _, _, err := run("completion", "tcsh"); err == nil {
		t.Error("unsupported shell should error")
	}
}

func TestListBadDateFlags(t *testing.T) {
	run, _ := harness(t)
	for _, args := range [][]string{
		{"deals", "list", "--close-before", "05/01/2026"},
		{"deals", "list", "--close-after", "nope"},
		{"calls", "list", "--since", "bad/date"},
		{"emails", "list", "--since", "13/13/13"},
		{"emails", "list", "--until", "xyz"},
	} {
		if _, _, err := run(args...); err == nil {
			t.Errorf("%v should reject the bad date", args)
		}
	}
}

func TestDealsListEmptyAmount(t *testing.T) {
	// fmtAmount returns "" when amount is zero — render a zero-amount deal.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":[{"crmId":"D9","name":"Free","status":"Open"}],"pagination":{}}`))
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
			_, _ = w.Write([]byte(`{"id":1,"title":"t"}`))
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
		{"calls", "open", "1"},
		{"deals", "open", "D1"},
		{"accounts", "open", "A1"},
	} {
		if err := run(args...); err == nil {
			t.Errorf("%v should error without a crmLink", args)
		}
	}
}

func TestEmailsGetTable(t *testing.T) {
	run, _ := harness(t)
	out, _, err := run("emails", "get", "5", "-o", "table")
	if err != nil || !strings.Contains(out, "Subject:") {
		t.Fatalf("emails get table: %v %s", err, out)
	}
}

func TestSummaryMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"title":"t"}`)) // no summary
	}))
	defer srv.Close()
	t.Setenv("MODJO_BASE_URL", srv.URL)
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	io, _, out, errOut := iostreams.Test()
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
