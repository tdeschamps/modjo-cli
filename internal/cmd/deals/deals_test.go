package deals

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

type req struct {
	method, path, body string
}

// dealServer records the last request and replies with the given body.
func dealServer(t *testing.T, status int, body string) (string, *req) {
	t.Helper()
	last := &req{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last.method, last.path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		last.body = string(b)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, last
}

func dealFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("MODJO_BASE_URL", baseURL)
	t.Setenv("MODJO_API_KEY", "k")
	io, _, out, errOut := iostreams.Test()
	return &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
	}, out, errOut
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdDeals(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestDealsSummary(t *testing.T) {
	url, last := dealServer(t, 200, `{"data":[{"type":"Overview","value":"Big deal in progress"}],"language":"en"}`)
	f, out, _ := dealFactory(t, url)
	f.Flags.Output = "table" // exercise the interactive branch
	if err := run(t, f, "summary", "99"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/deals/99/summary" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Big deal in progress") {
		t.Errorf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "Overview") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDealsSummaryJSON(t *testing.T) {
	url, _ := dealServer(t, 200, `{"data":[{"type":"Overview","value":"Big deal in progress"}],"language":"en"}`)
	f, out, _ := dealFactory(t, url)
	f.Flags.JSON = true
	if err := run(t, f, "summary", "99"); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "Big deal in progress") {
		t.Errorf("output = %q", o)
	}
	if !strings.Contains(o, "Overview") {
		t.Errorf("output = %q", o)
	}
}

func TestDealsSummaryEmpty(t *testing.T) {
	url, _ := dealServer(t, 200, `{"data":[],"language":"en"}`)
	f, out, errOut := dealFactory(t, url)
	f.Flags.Output = "table" // exercise the interactive branch
	if err := run(t, f, "summary", "99"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("stdout should be empty, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "No summary available for deal 99") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

// TestDealFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is covered without driving the wire.
func TestDealFieldExtractors(t *testing.T) {
	url, _ := dealServer(t, 200, `{}`)
	f, _, _ := dealFactory(t, url)
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{dealFields(f.IOStreams), api.Deal{
			ID: "12", Name: "Big deal", CRMLink: "https://crm/12", Status: api.StatusClosedWon,
			Amount: 1000, Currency: "EUR", Stage: "Negotiation", CloseDate: "2026-07-01",
		}},
		{dealSummaryFields(), api.DealSummaryBlock{Type: "Overview", Value: "All good"}},
	}
	for _, c := range cases {
		for _, fld := range c.fields {
			if fld.Name == "" || fld.Extract == nil {
				t.Errorf("bad field %+v", fld)
				continue
			}
			_ = fld.Extract(c.value) // must not panic
		}
	}
}

func TestDealsList(t *testing.T) {
	url, last := dealServer(t, 200, `{"data":[{"id":12,"name":"Big deal","status":"Open","stage":"Discovery"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out, _ := dealFactory(t, url)
	if err := run(t, f, "list", "--status", "open"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/deals" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Big deal") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDealsReadAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := dealServer(t, 500, `{"message":"boom"}`)
	f, _, _ := dealFactory(t, url)
	for _, args := range [][]string{
		{"list"}, {"summary", "12"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestDealsClientBuildErrors covers the APIClient() error branch in each RunE:
// a malformed config file makes the client fail to build before any request.
func TestDealsClientBuildErrors(t *testing.T) {
	url, _ := dealServer(t, 200, `{}`)
	f, _, _ := dealFactory(t, url)
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"}, {"summary", "12"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error from malformed config", args)
		}
	}
}
