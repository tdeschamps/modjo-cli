package callreviews

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

type req struct {
	method, path, query string
}

// reviewServer records the last request and replies with the given body.
func reviewServer(t *testing.T, status int, body string) (string, *req) {
	t.Helper()
	last := &req{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last.method, last.path, last.query = r.Method, r.URL.Path, r.URL.RawQuery
		_, _ = io.Copy(io.Discard, r.Body)
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

// reviewFactory pins the clock so relative dates like "30d" resolve
// deterministically.
func reviewFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
	t.Helper()
	t.Setenv("MODJO_BASE_URL", baseURL)
	t.Setenv("MODJO_API_KEY", "k")
	io, _, out, _ := iostreams.Test()
	return &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		ConfigPath: t.TempDir() + "/c.toml",
		Clock:      text.FixedClock(time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)),
	}, out
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdCallReviews(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

const reviewBody = `{"data":[{
	"id":1024,"callId":55555,"template":{"id":12,"title":"Cold Calling"},
	"revieweeId":88,"rating":3.2,"isPrivate":false,"isAiGenerated":false,
	"createdById":42,"createdOn":"2026-09-15T14:30:00.000Z",
	"answers":[{"id":42,"question":{"id":500,"title":"Intro"},"weight":0,"rating":3,"feedback":"Clean."}]
}],"pagination":{"page":1,"size":50,"total":1}}`

func TestCallReviewsList(t *testing.T) {
	url, last := reviewServer(t, 200, reviewBody)
	f, out := reviewFactory(t, url)
	if err := run(t, f, "list"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/call-reviews" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Cold Calling") {
		t.Errorf("output = %q", out.String())
	}
}

// --since/--until accept the same relative dates as `calls list` and land on
// the endpoint's camelCase params.
func TestCallReviewsListDateAndOrderFlags(t *testing.T) {
	url, last := reviewServer(t, 200, reviewBody)
	f, _ := reviewFactory(t, url)
	if err := run(t, f, "list", "--since", "30d", "--until", "2026-09-17", "--order", "asc"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"startDateTime=2026-08-18", "endDateTime=2026-09-17", "order=asc"} {
		if !strings.Contains(last.query, want) {
			t.Errorf("query = %q, want %s", last.query, want)
		}
	}
}

func TestCallReviewsListRejectsBadOrder(t *testing.T) {
	url, _ := reviewServer(t, 200, reviewBody)
	f, _ := reviewFactory(t, url)
	err := run(t, f, "list", "--order", "sideways")
	if err == nil {
		t.Fatal("expected a usage error for --order sideways")
	}
	if got := cmdutil.ExitCodeForError(err); got != cmdutil.ExitUsage {
		t.Errorf("exit code = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestCallReviewsListRejectsBadDate(t *testing.T) {
	url, _ := reviewServer(t, 200, reviewBody)
	f, _ := reviewFactory(t, url)
	for _, args := range [][]string{
		{"list", "--since", "yesterday-ish"},
		{"list", "--until", "yesterday-ish"},
	} {
		err := run(t, f, args...)
		if err == nil {
			t.Fatalf("%v: expected a usage error", args)
		}
		if got := cmdutil.ExitCodeForError(err); got != cmdutil.ExitUsage {
			t.Errorf("%v: exit code = %d, want %d", args, got, cmdutil.ExitUsage)
		}
	}
}

// The nested per-question answers are not a table column, but --json must carry
// them in full.
func TestCallReviewsListJSONKeepsAnswers(t *testing.T) {
	url, _ := reviewServer(t, 200, reviewBody)
	f, out := reviewFactory(t, url)
	f.Flags.JSON = true
	if err := run(t, f, "list"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"answers"`, `"Intro"`, `"Clean."`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("json output missing %s: %q", want, out.String())
		}
	}
}

// TestReviewFieldExtractors exercises every column's Extract closure so the
// table/csv path is covered without driving the wire.
func TestReviewFieldExtractors(t *testing.T) {
	v := api.CallReview{
		ID: "1", CallID: "2", Template: api.CallReviewRef{ID: "12", Title: "Cold Calling"},
		RevieweeID: "88", Rating: "3.2", IsAIGenerated: true, CreatedOn: "2026-09-15",
	}
	for _, f := range reviewFields() {
		if f.Name == "" || f.Extract == nil {
			t.Errorf("bad field %+v", f)
			continue
		}
		_ = f.Extract(v) // must not panic
	}
}

func TestCallReviewsListAPIError(t *testing.T) {
	url, _ := reviewServer(t, 500, `{"message":"boom"}`)
	f, _ := reviewFactory(t, url)
	if err := run(t, f, "list"); err == nil {
		t.Error("expected error on 500")
	}
}

// A malformed config makes APIClient() fail before any request, covering the
// client-build branch in RunE.
func TestCallReviewsListClientBuildError(t *testing.T) {
	url, _ := reviewServer(t, 200, reviewBody)
	f, _ := reviewFactory(t, url)
	if err := os.WriteFile(f.ConfigPath, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t, f, "list"); err == nil {
		t.Error("expected error from malformed config")
	}
}
