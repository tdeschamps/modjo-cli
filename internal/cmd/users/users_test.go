package users

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

// userServer records the last request and replies with the given body.
func userServer(t *testing.T, status int, body string) (string, *req) {
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

func userFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
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
	cmd := NewCmdUsers(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestUsersUpdate(t *testing.T) {
	url, last := userServer(t, 200, `{"id":7,"email":"a@x.com","name":"A B"}`)
	f, out := userFactory(t, url)
	if err := run(t, f, "update", "7", "--job-title", "VP Sales"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPatch || last.path != "/users/7" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"jobTitle":"VP Sales"`) {
		t.Errorf("body = %s", last.body)
	}
	if !strings.Contains(out.String(), "a@x.com") {
		t.Errorf("output = %q", out.String())
	}
}

func TestUsersUpdateSendsOnlyChangedFields(t *testing.T) {
	// A single --job-title must send only that field, not every other field as
	// an empty string.
	url, last := userServer(t, 200, `{"id":7,"email":"a@x.com"}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "update", "7", "--job-title", "VP"); err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{`"email"`, `"firstName"`, `"phoneNumber"`, `"role"`} {
		if strings.Contains(last.body, unwanted) {
			t.Errorf("unchanged field %s should not be sent: %s", unwanted, last.body)
		}
	}
}

func TestUsersUpdateCanClearField(t *testing.T) {
	// Passing --phone "" explicitly must send phoneNumber:"" so the server
	// clears it; an omitempty body would silently drop it (a no-op PATCH).
	url, last := userServer(t, 200, `{"id":7,"email":"a@x.com"}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "update", "7", "--phone", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(last.body, `"phoneNumber":""`) {
		t.Errorf("explicit empty phone must be sent: %s", last.body)
	}
}

func TestUsersUpdateRequiresAtLeastOneFlag(t *testing.T) {
	url, last := userServer(t, 200, `{}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "update", "7"); err == nil {
		t.Error("update without any flag should error")
	}
	if last.method != "" {
		t.Errorf("update with no flags must not call the API, got %s %s", last.method, last.path)
	}
}

func TestUsersUpdateDryRun(t *testing.T) {
	url, last := userServer(t, 200, `{}`)
	f, _ := userFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "update", "7", "--job-title", "VP Sales"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s %s", last.method, last.path)
	}
}

func TestUsersAddTeam(t *testing.T) {
	url, last := userServer(t, 201, `{"userId":7,"teamId":5}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "add-team", "7", "--team", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/users/7/teams" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"teamId":5`) {
		t.Errorf("body = %s", last.body)
	}
}

func TestUsersAddTeamRequiresTeam(t *testing.T) {
	url, last := userServer(t, 201, `{}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "add-team", "7"); err == nil {
		t.Error("add-team without --team should error")
	}
	if last.method != "" {
		t.Errorf("add-team with no --team must not call the API, got %s %s", last.method, last.path)
	}
}

func TestUsersAddTeamDryRun(t *testing.T) {
	url, last := userServer(t, 201, `{}`)
	f, _ := userFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "add-team", "7", "--team", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s %s", last.method, last.path)
	}
}

func TestUsersRemoveTeamWithYes(t *testing.T) {
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	f.Flags.Yes = true
	if err := run(t, f, "remove-team", "7", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodDelete || last.path != "/users/7/teams/5" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestUsersRemoveTeamDryRun(t *testing.T) {
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "remove-team", "7", "5"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

// TestUserFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is covered without driving the wire.
func TestUserFieldExtractors(t *testing.T) {
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{userFields(), api.User{ID: "7", Email: "a@x.com", Name: "A B", Role: "ADMIN", Department: "Sales", Title: "VP"}},
		{userTeamFields(), api.UserTeam{UserID: "7", TeamID: "5"}},
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

func TestUsersRemoveTeamAborts(t *testing.T) {
	// The test IOStreams never prompts, so Confirm returns the default (false):
	// removal must abort without calling the API.
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	if err := run(t, f, "remove-team", "7", "5"); err == nil {
		t.Error("expected aborted error when confirmation is declined")
	}
	if last.method != "" {
		t.Errorf("aborted remove must not call the API, got %s", last.method)
	}
}

func TestUsersDeleteAborts(t *testing.T) {
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	if err := run(t, f, "delete", "7"); err == nil {
		t.Error("expected aborted error when confirmation is declined")
	}
	if last.method != "" {
		t.Errorf("aborted delete must not call the API, got %s", last.method)
	}
}

func TestUsersList(t *testing.T) {
	url, last := userServer(t, 200, `{"data":[{"id":7,"email":"a@x.com","firstName":"A","lastName":"B"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := userFactory(t, url)
	if err := run(t, f, "list", "--email", "a@x.com"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/users" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "a@x.com") {
		t.Errorf("output = %q", out.String())
	}
}

func TestUsersGet(t *testing.T) {
	url, last := userServer(t, 200, `{"id":7,"email":"a@x.com","firstName":"A","lastName":"B"}`)
	f, out := userFactory(t, url)
	if err := run(t, f, "get", "7"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/users/7" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "a@x.com") {
		t.Errorf("output = %q", out.String())
	}
}

func TestUsersCreate(t *testing.T) {
	url, last := userServer(t, 201, `{"id":7,"email":"a@x.com","firstName":"A","lastName":"B"}`)
	f, out := userFactory(t, url)
	if err := run(t, f, "create", "--first-name", "A", "--last-name", "B", "--email", "a@x.com"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/users" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "a@x.com") {
		t.Errorf("output = %q", out.String())
	}
}

func TestUsersCreateRequiresFlags(t *testing.T) {
	url, last := userServer(t, 201, `{}`)
	f, _ := userFactory(t, url)
	if err := run(t, f, "create", "--first-name", "A"); err == nil {
		t.Error("create without required flags should error")
	}
	if last.method != "" {
		t.Errorf("incomplete create must not call the API, got %s", last.method)
	}
}

func TestUsersCreateDryRun(t *testing.T) {
	url, last := userServer(t, 201, `{}`)
	f, _ := userFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "create", "--first-name", "A", "--last-name", "B", "--email", "a@x.com"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestUsersDeleteWithYes(t *testing.T) {
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	f.Flags.Yes = true
	if err := run(t, f, "delete", "7"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodDelete || last.path != "/users/7" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestUsersDeleteDryRun(t *testing.T) {
	url, last := userServer(t, 204, "")
	f, _ := userFactory(t, url)
	f.Flags.DryRun = true
	if err := run(t, f, "delete", "7"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestUsersReadAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := userServer(t, 500, `{"message":"boom"}`)
	f, _ := userFactory(t, url)
	for _, args := range [][]string{
		{"list"}, {"get", "7"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestUsersWriteAPIErrors covers the write-call error branches: create/update/
// add-team/remove-team/delete each surface a non-2xx response from the server.
func TestUsersWriteAPIErrors(t *testing.T) {
	url, _ := userServer(t, 500, `{"message":"boom"}`)
	f, _ := userFactory(t, url)
	f.Flags.Yes = true // let remove-team/delete reach the API
	for _, args := range [][]string{
		{"create", "--first-name", "A", "--last-name", "B", "--email", "a@x.com"},
		{"update", "7", "--job-title", "VP"},
		{"add-team", "7", "--team", "5"},
		{"remove-team", "7", "5"},
		{"delete", "7"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

// TestUsersClientBuildErrors covers the APIClient() error branch in each RunE:
// a malformed config file makes the client fail to build before any request.
func TestUsersClientBuildErrors(t *testing.T) {
	url, _ := userServer(t, 200, `{}`)
	f, _ := userFactory(t, url)
	f.Flags.Yes = true // skip confirm prompts so remove-team/delete reach APIClient()
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list"},
		{"get", "7"},
		{"create", "--first-name", "A", "--last-name", "B", "--email", "a@x.com"},
		{"update", "7", "--job-title", "VP"},
		{"add-team", "7", "--team", "5"},
		{"remove-team", "7", "5"},
		{"delete", "7"},
	} {
		if err := run(t, f, args...); err == nil {
			t.Errorf("%v: expected error from malformed config", args)
		}
	}
}
