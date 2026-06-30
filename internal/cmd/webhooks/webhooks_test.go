package webhooks

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

// webhookServer records the last request and replies with the given body.
func webhookServer(t *testing.T, status int, body string) (string, *req) {
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

func webhookFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
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
	cmd := NewCmdWebhooks(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestWebhooksUpdate(t *testing.T) {
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"Renamed","url":"https://x","events":["call_summarized"]}`)
	f, out := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1", "--name", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPatch || last.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"name":"Renamed"`) {
		t.Errorf("body = %s", last.body)
	}
	if !strings.Contains(out.String(), "Renamed") {
		t.Errorf("output = %q", out.String())
	}
}

func TestWebhooksUpdateEvents(t *testing.T) {
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"N","url":"https://x","events":["call_summarized"]}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1", "--event", "call_summarized"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPatch || last.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"events":["call_summarized"]`) {
		t.Errorf("body = %s", last.body)
	}
}

func TestWebhooksUpdateRequiresAFlag(t *testing.T) {
	url, last := webhookServer(t, 200, `{}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1"); err == nil {
		t.Error("update with no flags should error")
	}
	if last.method != "" {
		t.Errorf("update with no flags must not call the API, got %s %s", last.method, last.path)
	}
}

func TestWebhooksUpdateAcceptsExplicitEmptyName(t *testing.T) {
	// Passing --name "" is a provided flag (the value, not its emptiness, is
	// what matters): it must be accepted and sent as name:"" rather than
	// rejected as "no flags given".
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"","url":"https://x"}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1", "--name", ""); err != nil {
		t.Fatalf("update --name \"\" should be accepted: %v", err)
	}
	if last.method != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", last.method)
	}
	if !strings.Contains(last.body, `"name":""`) {
		t.Errorf("explicit empty name must be sent: %s", last.body)
	}
}

func TestWebhooksUpdateSendsOnlyChangedFields(t *testing.T) {
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"N","url":"https://x"}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1", "--name", "N"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(last.body, `"url"`) || strings.Contains(last.body, `"events"`) {
		t.Errorf("unchanged url/events should not be sent: %s", last.body)
	}
}

func TestWebhooksUpdateURL(t *testing.T) {
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"N","url":"https://new"}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "update", "wh-1", "--url", "https://new"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPatch || last.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"url":"https://new"`) {
		t.Errorf("changed url must be sent: %s", last.body)
	}
	if strings.Contains(last.body, `"name"`) {
		t.Errorf("unchanged name should not be sent: %s", last.body)
	}
}

func TestWebhooksUpdateURLDryRun(t *testing.T) {
	url, last := webhookServer(t, 200, `{}`)
	f, _ := webhookFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "update", "wh-1", "--url", "https://new"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestWebhooksUpdateDryRun(t *testing.T) {
	url, last := webhookServer(t, 200, `{}`)
	f, _ := webhookFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "update", "wh-1", "--name", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s %s", last.method, last.path)
	}
}

// TestWebhookFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is covered without driving the wire.
func TestWebhookFieldExtractors(t *testing.T) {
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{webhookFields(), api.Webhook{UUID: "wh-1", Name: "N", URL: "https://x", Events: []string{"call_summarized", "call_recording_deleted"}}},
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

func TestWebhooksDeleteAborts(t *testing.T) {
	// The test IOStreams never prompts, so Confirm returns the default (false):
	// deletion must abort without calling the API.
	url, last := webhookServer(t, 204, "")
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "delete", "wh-1"); err == nil {
		t.Error("expected aborted error when confirmation is declined")
	}
	if last.method != "" {
		t.Errorf("aborted delete must not call the API, got %s", last.method)
	}
}

func TestWebhooksList(t *testing.T) {
	url, last := webhookServer(t, 200, `{"data":[{"uuid":"wh-1","name":"N","url":"https://x","events":["call_summarized"]}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := webhookFactory(t, url)
	if err := run(t, f, "list"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/webhooks" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "wh-1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestWebhooksGet(t *testing.T) {
	url, last := webhookServer(t, 200, `{"uuid":"wh-1","name":"N","url":"https://x","events":["call_summarized"]}`)
	f, out := webhookFactory(t, url)
	if err := run(t, f, "get", "wh-1"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "wh-1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestWebhooksCreate(t *testing.T) {
	url, last := webhookServer(t, 201, `{"uuid":"wh-1","name":"N","url":"https://x","events":["call_summarized"]}`)
	f, out := webhookFactory(t, url)
	if err := run(t, f, "create", "--name", "N", "--url", "https://x", "--event", "call_summarized"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/webhooks" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "wh-1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestWebhooksCreateRequiresFlags(t *testing.T) {
	url, last := webhookServer(t, 201, `{}`)
	f, _ := webhookFactory(t, url)
	if err := run(t, f, "create", "--name", "N"); err == nil {
		t.Error("create without required flags should error")
	}
	if last.method != "" {
		t.Errorf("incomplete create must not call the API, got %s", last.method)
	}
}

func TestWebhooksCreateDryRun(t *testing.T) {
	url, last := webhookServer(t, 201, `{}`)
	f, _ := webhookFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "create", "--name", "N", "--url", "https://x", "--event", "call_summarized"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestWebhooksDeleteWithYes(t *testing.T) {
	url, last := webhookServer(t, 204, "")
	f, _ := webhookFactory(t, url)
	f.Flags.Yes = true
	if err := run(t, f, "delete", "wh-1"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodDelete || last.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestWebhooksDeleteDryRun(t *testing.T) {
	url, last := webhookServer(t, 204, "")
	f, _ := webhookFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "delete", "wh-1"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestWebhooksReadAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := webhookServer(t, 500, `{"message":"boom"}`)
	f, _ := webhookFactory(t, url)
	for _, args := range [][]string{
		{"list"}, {"get", "wh-1"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestWebhooksWriteAPIErrors covers the write-call error branches: create/
// update/delete each surface a non-2xx response from the server.
func TestWebhooksWriteAPIErrors(t *testing.T) {
	url, _ := webhookServer(t, 500, `{"message":"boom"}`)
	f, _ := webhookFactory(t, url)
	f.Flags.Yes = true // let delete reach the API
	for _, args := range [][]string{
		{"create", "--name", "N", "--url", "https://x", "--event", "call_summarized"},
		{"update", "wh-1", "--name", "X"},
		{"delete", "wh-1"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestWebhooksClientBuildErrors covers the APIClient() error branch in each
// RunE: a malformed config file makes the client fail to build before any
// request.
func TestWebhooksClientBuildErrors(t *testing.T) {
	url, _ := webhookServer(t, 200, `{}`)
	f, _ := webhookFactory(t, url)
	f.Flags.Yes = true // skip the confirm prompt so delete reaches APIClient()
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"},
		{"get", "wh-1"},
		{"create", "--name", "N", "--url", "https://x", "--event", "call_summarized"},
		{"update", "wh-1", "--name", "X"},
		{"delete", "wh-1"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error from malformed config", args)
		}
	}
}
