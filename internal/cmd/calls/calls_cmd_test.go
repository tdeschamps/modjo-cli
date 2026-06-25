package calls

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

type callsReq struct {
	method, path, body string
}

// callsCmdServer records the last request and replies with the given body.
func callsCmdServer(t *testing.T, status int, body string) (string, *callsReq) {
	t.Helper()
	last := &callsReq{}
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

func callsCmdFactory(t *testing.T, baseURL string) (*cmdutil.Factory, *bytes.Buffer) {
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

func runCalls(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdCalls(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestCallsNotes(t *testing.T) {
	url, last := callsCmdServer(t, 200, `{"data":[{"id":7,"title":"Recap","status":"PUBLISHED","type":"AI","date":"2026-06-01"}]}`)
	f, out := callsCmdFactory(t, url)
	if err := runCalls(t, f, "notes", "42"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/calls/42/notes" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Recap") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCallsNextSteps(t *testing.T) {
	url, last := callsCmdServer(t, 200, `{"data":[{"title":"Send proposal","description":"by Friday"}]}`)
	f, out := callsCmdFactory(t, url)
	if err := runCalls(t, f, "next-steps", "42"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/calls/42/next-steps" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Send proposal") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCallsTagsList(t *testing.T) {
	url, last := callsCmdServer(t, 200, `{"data":[{"id":3,"name":"Demo","color":"#fff"}]}`)
	f, out := callsCmdFactory(t, url)
	if err := runCalls(t, f, "tags", "list", "42"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodGet || last.path != "/calls/42/tags" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(out.String(), "Demo") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCallsCrmAnswers(t *testing.T) {
	url, last := callsCmdServer(t, 200, `{"data":[{"uuid":"ans-1","crmFillingFieldUuid":"f-9","crmId":"crm-5","modifiedOn":"2026-06-02"}],"pagination":{"page":1,"size":50,"total":1}}`)
	f, out := callsCmdFactory(t, url)
	if err := runCalls(t, f, "crm-answers", "42"); err != nil {
		t.Fatal(err)
	}
	if last.path != "/calls/42/crm-filling-answers" {
		t.Errorf("path = %s", last.path)
	}
	if !strings.Contains(out.String(), "ans-1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCallsTagsAdd(t *testing.T) {
	url, last := callsCmdServer(t, 201, `{"callId":42,"tagId":3}`)
	f, _ := callsCmdFactory(t, url)
	if err := runCalls(t, f, "tags", "add", "42", "--tag", "3"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/calls/42/tags" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"tagId":3`) {
		t.Errorf("body = %s", last.body)
	}
}

func TestCallsTagsAddRequiresTag(t *testing.T) {
	url, _ := callsCmdServer(t, 201, `{}`)
	f, _ := callsCmdFactory(t, url)
	if err := runCalls(t, f, "tags", "add", "42"); err == nil {
		t.Error("add without --tag should error")
	}
}

func TestCallsTagsAddDryRun(t *testing.T) {
	url, last := callsCmdServer(t, 201, `{}`)
	f, _ := callsCmdFactory(t, url)
	f.Flags.DryRun = true
	if err := runCalls(t, f, "tags", "add", "42", "--tag", "3"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s %s", last.method, last.path)
	}
}

func TestCallsTagsRemoveWithYes(t *testing.T) {
	url, last := callsCmdServer(t, 204, "")
	f, _ := callsCmdFactory(t, url)
	f.Flags.Yes = true
	if err := runCalls(t, f, "tags", "remove", "42", "3"); err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodDelete || last.path != "/calls/42/tags/3" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
}

func TestCallsTagsRemoveDryRun(t *testing.T) {
	url, last := callsCmdServer(t, 204, "")
	f, _ := callsCmdFactory(t, url)
	f.Flags.DryRun = true
	if err := runCalls(t, f, "tags", "remove", "42", "3"); err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}

func TestCallsTagsRemoveAborts(t *testing.T) {
	// The test IOStreams never prompts, so Confirm returns the default (false):
	// removal must abort without calling the API.
	url, last := callsCmdServer(t, 204, "")
	f, _ := callsCmdFactory(t, url)
	if err := runCalls(t, f, "tags", "remove", "42", "3"); err == nil {
		t.Error("expected aborted error when confirmation is declined")
	}
	if last.method != "" {
		t.Errorf("aborted remove must not call the API, got %s", last.method)
	}
}

func TestCallsSubresourceAPIErrors(t *testing.T) {
	// Each read surfaces a non-2xx API error rather than swallowing it.
	url, _ := callsCmdServer(t, 500, `{"message":"boom"}`)
	f, _ := callsCmdFactory(t, url)
	for _, args := range [][]string{
		{"notes", "42"}, {"next-steps", "42"}, {"crm-answers", "42"}, {"tags", "list", "42"},
	} {
		if err := runCalls(t, f, args...); err == nil {
			t.Errorf("%v: expected error on 500", args)
		}
	}
}

func TestCallsUpload(t *testing.T) {
	url, last := callsCmdServer(t, 202, `{"callId":"c-1","status":"processing"}`)
	f, out := callsCmdFactory(t, url)
	err := runCalls(t, f,
		"upload",
		"--media-url", "https://media/x.mp3",
		"--date", "2026-06-25T10:00:00Z",
		"--name", "Discovery",
		"--direction", "inbound",
		"--duration", "600",
		"--participant", "rep@acme.com:user:Rep",
		"--participant", "lead@buyer.com:contact",
		"--tag", "Pricing",
		"--account", "salesforce:A1",
		"--deal", "salesforce:D1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if last.method != http.MethodPost || last.path != "/calls" {
		t.Errorf("request = %s %s", last.method, last.path)
	}
	if !strings.Contains(last.body, `"downloadMediaUrl":"https://media/x.mp3"`) {
		t.Errorf("media url missing: %s", last.body)
	}
	if !strings.Contains(last.body, `"email":"rep@acme.com"`) || !strings.Contains(last.body, `"type":"contact"`) {
		t.Errorf("participants missing: %s", last.body)
	}
	if !strings.Contains(last.body, `"crmId":"A1"`) {
		t.Errorf("account ref missing: %s", last.body)
	}
	if !strings.Contains(out.String(), "c-1") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCallsUploadRequiresMediaAndParticipant(t *testing.T) {
	url, _ := callsCmdServer(t, 202, `{}`)
	f, _ := callsCmdFactory(t, url)
	// Missing --media-url and --participant.
	if err := runCalls(t, f, "upload", "--date", "2026-06-25T10:00:00Z"); err == nil {
		t.Error("upload without --media-url/--participant should error")
	}
}

func TestCallsUploadBadParticipant(t *testing.T) {
	url, _ := callsCmdServer(t, 202, `{}`)
	f, _ := callsCmdFactory(t, url)
	err := runCalls(t, f,
		"upload",
		"--media-url", "https://x",
		"--date", "2026-06-25T10:00:00Z",
		"--participant", "no-type-here",
	)
	if err == nil {
		t.Error("malformed --participant should error")
	}
}

// TestCallFieldExtractors exercises every column's Extract closure so the
// renderer's table/csv path is fully covered without driving the wire.
func TestCallFieldExtractors(t *testing.T) {
	cases := []struct {
		fields []output.Field
		value  any
	}{
		{noteFields(), api.Note{ID: "9", Title: "T", Status: "PUBLISHED", Type: "AI", Date: "2026-06-01"}},
		{nextStepFields(), api.NextStepItem{Title: "Step", Description: "Do the thing"}},
		{tagFields(), api.Tag{ID: "3", Name: "Pricing", Color: "blue"}},
		{callTagFields(), api.CallTag{CallID: "42", TagID: "3"}},
		{crmAnswerFields(), api.CrmFillingAnswer{UUID: "a1", CrmFillingFieldUUID: "f1", CRMID: "C1", ModifiedOn: "2026-06-02"}},
		{uploadFields(), api.UploadCallResponse{CallID: "c1", Status: "processing"}},
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

func TestParseParticipantsAndCRMRef(t *testing.T) {
	if _, err := parseParticipants([]string{"e@x.com:user:Name", "c@x.com:contact"}); err != nil {
		t.Fatalf("valid participants: %v", err)
	}
	for _, bad := range []string{"noseparator", "e@x.com:badtype", ":user", "e@x.com:"} {
		if _, err := parseParticipants([]string{bad}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	if ref, err := parseCRMRef(""); err != nil || ref != nil {
		t.Errorf("empty spec → nil,nil; got %v,%v", ref, err)
	}
	if ref, err := parseCRMRef("salesforce:A1"); err != nil || ref == nil || ref.CRMID != "A1" {
		t.Errorf("good ref = %v,%v", ref, err)
	}
	for _, bad := range []string{"noseparator", "crmonly:", ":idonly"} {
		if _, err := parseCRMRef(bad); err == nil {
			t.Errorf("expected error for crm ref %q", bad)
		}
	}
}

func TestCallsUploadRejectsBadDirection(t *testing.T) {
	url, last := callsCmdServer(t, 202, `{}`)
	f, _ := callsCmdFactory(t, url)
	err := runCalls(t, f,
		"upload",
		"--media-url", "https://x",
		"--date", "2026-06-25T10:00:00Z",
		"--participant", "rep@acme.com:user",
		"--direction", "sideways",
	)
	if err == nil {
		t.Error("invalid --direction should error before calling the API")
	}
	if last.method != "" {
		t.Errorf("bad direction must not call the API, got %s", last.method)
	}
}

func TestCallsUploadDryRun(t *testing.T) {
	url, last := callsCmdServer(t, 202, `{}`)
	f, _ := callsCmdFactory(t, url)
	f.Flags.DryRun = true
	err := runCalls(t, f,
		"upload",
		"--media-url", "https://x",
		"--date", "2026-06-25T10:00:00Z",
		"--participant", "rep@acme.com:user",
	)
	if err != nil {
		t.Fatal(err)
	}
	if last.method != "" {
		t.Errorf("dry-run must not call the API, got %s", last.method)
	}
}
