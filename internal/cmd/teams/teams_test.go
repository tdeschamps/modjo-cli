package teams

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

// teamServer records the last request and replies with the given body.
func teamServer(t *testing.T, status int, body string) (string, *req) {
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

func teamFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
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
	cmd := NewCmdTeams(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestTeamsCreate(t *testing.T) {
	url, last := teamServer(t, 201, `{"id":5,"name":"Sales"}`)
	f, out := teamFactory(t, url)
	if err := run(t, f, "create", "--name", "Sales"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/teams" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"name":"Sales"`) {
		t.Errorf("body = %s", last.body)
	}
	if !strings.Contains(out.String(), "Sales") {
		t.Errorf("output = %q", out.String())
	}
}

func TestTeamsCreateRequiresName(t *testing.T) {
	url, _ := teamServer(t, 201, `{}`)
	f, _ := teamFactory(t, url)
	if err := run(t, f, "create"); err == nil {
		t.Error("create without --name should error")
	}
}

func TestTeamsCreateDryRun(t *testing.T) {
	url, last := teamServer(t, 201, `{}`)
	f, _ := teamFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "create", "--name", "Sales"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s %s", last.method, last.path)
	}
}

func TestTeamsUpdate(t *testing.T) {
	url, last := teamServer(t, 200, `{"id":5,"name":"Renamed"}`)
	f, _ := teamFactory(t, url)
	if err := run(t, f, "update", "5", "--name", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPatch || last.path != "/teams/5" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestTeamsDeleteWithYes(t *testing.T) {
	url, last := teamServer(t, 204, "")
	f, _ := teamFactory(t, url)
	f.Flags.Yes = true
	if err := run(t, f, "delete", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodDelete || last.path != "/teams/5" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestTeamsDeleteDryRun(t *testing.T) {
	url, last := teamServer(t, 204, "")
	f, _ := teamFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "delete", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestTeamsMembers(t *testing.T) {
	url, last := teamServer(t, 200, `{"data":[{"id":1,"email":"a@x.com","firstName":"A","lastName":"B"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := teamFactory(t, url)
	if err := run(t, f, "members", "5"); err != nil {
		t.Fatal(err)
	}
	if last.path != "/teams/5/members" {
		t.Errorf("path = %s", last.path)
	}
	if !strings.Contains(out.String(), "a@x.com") {
		t.Errorf("output = %q", out.String())
	}
}

// TestTeamFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is covered without driving the wire.
func TestTeamFieldExtractors(t *testing.T) {
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{teamFields(), api.Team{ID: "5", Name: "Sales", CreatedOn: "2026-06-01", ModifiedOn: "2026-06-02"}},
		{memberFields(), api.TeamMember{ID: "1", Email: "a@x.com", FirstName: "A", LastName: "B", Role: "ADMIN"}},
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

func TestTeamsDeleteAborts(t *testing.T) {
	// The test IOStreams never prompts, so Confirm returns the default (false):
	// deletion must abort without calling the API.
	url, last := teamServer(t, 204, "")
	f, _ := teamFactory(t, url)
	if err := run(t, f, "delete", "5"); err == nil {
		t.Error("expected aborted error when confirmation is declined")
	}
	if last.method != "" {
		t.Errorf("aborted delete must not call the API, got %s", last.method)
	}
}

func TestTeamsList(t *testing.T) {
	url, last := teamServer(t, 200, `{"data":[{"id":5,"name":"Sales"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := teamFactory(t, url)
	if err := run(t, f, "list", "--name", "Sales"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/teams" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Sales") {
		t.Errorf("output = %q", out.String())
	}
}

func TestTeamsGet(t *testing.T) {
	url, last := teamServer(t, 200, `{"id":5,"name":"Sales"}`)
	f, out := teamFactory(t, url)
	if err := run(t, f, "get", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/teams/5" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Sales") {
		t.Errorf("output = %q", out.String())
	}
}

func TestTeamsReadAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := teamServer(t, 500, `{"message":"boom"}`)
	f, _ := teamFactory(t, url)
	for _, args := range [][]string{
		{"list"}, {"get", "5"}, {"members", "5"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

func TestTeamsUpdateRequiresName(t *testing.T) {
	url, last := teamServer(t, 200, `{}`)
	f, _ := teamFactory(t, url)
	if err := run(t, f, "update", "5"); err == nil {
		t.Error("update without --name should error")
	}
	if last.method != "" {
		t.Errorf("update with no --name must not call the API, got %s", last.method)
	}
}

func TestTeamsUpdateDryRun(t *testing.T) {
	url, last := teamServer(t, 200, `{}`)
	f, _ := teamFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "update", "5", "--name", "X"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

// TestTeamsWriteAPIErrors covers the write-call error branches: create/update/
// delete each surface a non-2xx response from the server.
func TestTeamsWriteAPIErrors(t *testing.T) {
	url, _ := teamServer(t, 500, `{"message":"boom"}`)
	f, _ := teamFactory(t, url)
	f.Flags.Yes = true // let delete reach the API
	for _, args := range [][]string{
		{"create", "--name", "X"}, {"update", "5", "--name", "X"}, {"delete", "5"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestTeamsClientBuildErrors covers the APIClient() error branch in each RunE:
// a malformed config file makes the client fail to build before any request.
func TestTeamsClientBuildErrors(t *testing.T) {
	url, _ := teamServer(t, 200, `{}`)
	f, _ := teamFactory(t, url)
	f.Flags.Yes = true // skip the confirm prompt so delete reaches APIClient()
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"},
		{"get", "5"},
		{"members", "5"},
		{"create", "--name", "X"},
		{"update", "5", "--name", "X"},
		{"delete", "5"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error from malformed config", args)
		}
	}
}
