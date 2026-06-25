package crmtemplates

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
	method, path, query, body string
}

// crmTemplateServer records the last request and replies with the given body.
func crmTemplateServer(t *testing.T, status int, body string) (string, *req) {
	t.Helper()
	last := &req{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last.method, last.path, last.query = r.Method, r.URL.Path, r.URL.RawQuery
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

func crmTemplateFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
	t.Helper()
	t.Setenv("MODJO_BASE_URL", baseURL)
	t.Setenv("MODJO_API_KEY", "k")
	io, _, out, _ := iostreams.Test()
	return &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
	}, out
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdCrmTemplates(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestCrmTemplatesList(t *testing.T) {
	url, last := crmTemplateServer(t, 200, `{"data":[{"uuid":"t1","title":"Discovery","status":"published","language":"en"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := crmTemplateFactory(t, url)
	if err := run(t, f, "list", "--status", "published"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/crm-filling-templates" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.query, "status=published") {
		t.Errorf("status query = %q", last.query)
	}
	if !strings.Contains(out.String(), "t1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCrmTemplatesGet(t *testing.T) {
	url, last := crmTemplateServer(t, 200, `{"uuid":"t1","title":"Discovery","status":"published","language":"en"}`)
	f, out := crmTemplateFactory(t, url)
	if err := run(t, f, "get", "t1"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/crm-filling-templates/t1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Discovery") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCrmTemplatesFields(t *testing.T) {
	url, last := crmTemplateServer(t, 200, `{"data":[{"uuid":"f1","order":1,"crm":"hubspot","entityType":"deal","fieldKey":"amount","fieldType":"number","prompt":"Deal value"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := crmTemplateFactory(t, url)
	if err := run(t, f, "fields", "t1"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/crm-filling-templates/t1/fields" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "amount") {
		t.Errorf("output = %q", out.String())
	}
}

// TestCrmTemplateFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is covered without driving the wire.
func TestCrmTemplateFieldExtractors(t *testing.T) {
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{templateFields(), api.CrmFillingTemplate{UUID: "t1", Title: "Discovery", Status: "published", Language: "en"}},
		{fieldFields(), api.CrmFillingField{
			UUID: "f1", Order: "1", CRM: "hubspot", EntityType: "deal",
			FieldKey: "amount", FieldType: "number", Prompt: "Deal value",
		}},
	}
	for _, c := range cases {
		for _, f := range c.fields {
			if f.Name == "" || f.Extract == nil {
				t.Errorf("bad field %+v", f)
				continue
			}
			_ = f.Extract(c.value) // must not panic
		}
	}
}

func TestCrmTemplatesReadAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := crmTemplateServer(t, 500, `{"message":"boom"}`)
	f, _ := crmTemplateFactory(t, url)
	for _, args := range [][]string{
		{"list"}, {"get", "t1"}, {"fields", "t1"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestCrmTemplatesClientBuildErrors covers the APIClient() error branch in each
// RunE: a malformed config file makes the client fail to build before any
// request.
func TestCrmTemplatesClientBuildErrors(t *testing.T) {
	url, _ := crmTemplateServer(t, 200, `{}`)
	f, _ := crmTemplateFactory(t, url)
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"}, {"get", "t1"}, {"fields", "t1"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error from malformed config", args)
		}
	}
}
