package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// recordingServer captures the method, path, query and body of the single
// request a client method makes, then replies with the supplied status+body.
// It asserts the bearer token is injected. Returned *capture is filled in after
// the client call completes.
type capture struct {
	method string
	path   string
	query  string
	body   string
}

func recordingServer(t *testing.T, status int, respBody string) (*httptest.Server, *capture) {
	t.Helper()
	cap := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.query = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		cap.body = string(b)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		if respBody != "" {
			_, _ = io.WriteString(w, respBody)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

// --- call sub-resources (non-paginated {data:[...]}) ---

func TestGetCallNotes(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"id":7,"title":"Recap","status":"PUBLISHED","type":"AI"}]}`)
	c := newTestClient(srv.URL)
	notes, err := c.GetCallNotes(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodGet || cap.path != "/calls/42/notes" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if len(notes) != 1 || notes[0].Title != "Recap" {
		t.Fatalf("notes = %+v", notes)
	}
}

func TestGetCallNextSteps(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"title":"Send quote","description":"by Friday"}]}`)
	c := newTestClient(srv.URL)
	steps, err := c.GetCallNextSteps(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if cap.path != "/calls/42/next-steps" {
		t.Errorf("path = %s", cap.path)
	}
	if len(steps) != 1 || steps[0].Title != "Send quote" {
		t.Fatalf("steps = %+v", steps)
	}
}

func TestGetCallTags(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"id":3,"name":"Pricing"}]}`)
	c := newTestClient(srv.URL)
	tags, err := c.GetCallTags(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if cap.path != "/calls/42/tags" {
		t.Errorf("path = %s", cap.path)
	}
	if len(tags) != 1 || tags[0].Name != "Pricing" {
		t.Fatalf("tags = %+v", tags)
	}
}

// --- call tag writes ---

func TestAddCallTag(t *testing.T) {
	srv, cap := recordingServer(t, 201, `{"callId":42,"tagId":3}`)
	c := newTestClient(srv.URL)
	ct, err := c.AddCallTag(context.Background(), "42", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/calls/42/tags" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(cap.body), &sent); err != nil {
		t.Fatalf("body not json: %q", cap.body)
	}
	if sent["tagId"] != float64(3) {
		t.Errorf("body tagId = %v, want 3 (body=%s)", sent["tagId"], cap.body)
	}
	if ct.TagID.String() != "3" || ct.CallID.String() != "42" {
		t.Errorf("CallTag = %+v", ct)
	}
}

func TestRemoveCallTag(t *testing.T) {
	srv, cap := recordingServer(t, 204, "")
	c := newTestClient(srv.URL)
	if err := c.RemoveCallTag(context.Background(), "42", "3"); err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodDelete || cap.path != "/calls/42/tags/3" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
}

// --- call upload (202) ---

func TestUploadCall(t *testing.T) {
	srv, cap := recordingServer(t, 202, `{"callId":"c-uuid-1","status":"processing"}`)
	c := newTestClient(srv.URL)
	in := UploadCallInput{
		DownloadMediaURL: "https://media/x.mp3",
		Date:             "2026-06-25T10:00:00Z",
		Name:             "Discovery",
		Direction:        "inbound",
		Duration:         600,
		Participants: []UploadCallParticipant{
			{Email: "rep@acme.com", Type: "user", Name: "Rep"},
			{Email: "lead@buyer.com", Type: "contact"},
		},
		Tags:    []string{"Pricing"},
		Account: &CRMRef{CRM: "salesforce", CRMID: "A1"},
		Deal:    &CRMRef{CRM: "salesforce", CRMID: "D1"},
	}
	resp, err := c.UploadCall(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/calls" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if resp.CallID != "c-uuid-1" || resp.Status != "processing" {
		t.Errorf("resp = %+v", resp)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(cap.body), &sent); err != nil {
		t.Fatalf("body not json: %q", cap.body)
	}
	if sent["downloadMediaUrl"] != "https://media/x.mp3" {
		t.Errorf("downloadMediaUrl missing: %s", cap.body)
	}
	parts, _ := sent["participants"].([]any)
	if len(parts) != 2 {
		t.Errorf("participants = %v", sent["participants"])
	}
}

// --- deal summary (single object) ---

func TestGetDealSummary(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"type":"overview","value":"Good call"}],"language":"en"}`)
	c := newTestClient(srv.URL)
	sum, err := c.GetDealSummary(context.Background(), "99")
	if err != nil {
		t.Fatal(err)
	}
	if cap.path != "/deals/99/summary" {
		t.Errorf("path = %s", cap.path)
	}
	if sum.Language != "en" || len(sum.Data) != 1 || sum.Data[0].Type != "overview" {
		t.Fatalf("summary = %+v", sum)
	}
}

// --- crm filling answers (paginated, per call) ---

func TestCrmFillingAnswers_Paginated(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"uuid":"a1","callId":42,"crmFillingFieldUuid":"f1","crmId":"C1"}],"pagination":{"page":1,"size":50,"total":1}}`)
	c := newTestClient(srv.URL)
	var got []CrmFillingAnswer
	for a, err := range c.CrmFillingAnswers(context.Background(), "42", PageFilter{}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, a)
	}
	if cap.path != "/calls/42/crm-filling-answers" {
		t.Errorf("path = %s", cap.path)
	}
	if len(got) != 1 || got[0].UUID != "a1" {
		t.Fatalf("answers = %+v", got)
	}
}

// --- crm filling templates ---

func TestCrmFillingTemplates_ListWithStatusFilter(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"uuid":"t1","title":"T","status":"published"}],"pagination":{"page":1,"size":50,"total":1}}`)
	c := newTestClient(srv.URL)
	var got []CrmFillingTemplate
	for tpl, err := range c.CrmFillingTemplates(context.Background(), CrmFillingTemplateFilter{Status: "published"}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, tpl)
	}
	if cap.path != "/crm-filling-templates" {
		t.Errorf("path = %s", cap.path)
	}
	q, _ := url.ParseQuery(cap.query)
	if q.Get("status") != "published" {
		t.Errorf("status filter not sent: %q", cap.query)
	}
	if len(got) != 1 || got[0].UUID != "t1" {
		t.Fatalf("templates = %+v", got)
	}
}

func TestGetCrmFillingTemplate(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"uuid":"t1","title":"T","status":"published"}`)
	c := newTestClient(srv.URL)
	tpl, err := c.GetCrmFillingTemplate(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if cap.path != "/crm-filling-templates/t1" {
		t.Errorf("path = %s", cap.path)
	}
	if tpl.Title != "T" {
		t.Fatalf("template = %+v", tpl)
	}
}

func TestCrmFillingTemplateFields_Paginated(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"uuid":"f1","order":1,"prompt":"Q","crm":"hubspot","fieldKey":"k"}],"pagination":{"page":1,"size":50,"total":1}}`)
	c := newTestClient(srv.URL)
	var got []CrmFillingField
	for f, err := range c.CrmFillingTemplateFields(context.Background(), "t1", PageFilter{}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	if cap.path != "/crm-filling-templates/t1/fields" {
		t.Errorf("path = %s", cap.path)
	}
	if len(got) != 1 || got[0].FieldKey != "k" {
		t.Fatalf("fields = %+v", got)
	}
}

// CrmFillingField.IsActive/IsAutoPush are spec-required booleans, so a false
// value must serialize to `false` in `-o json`, not vanish. (With `,omitempty`
// the keys disappear and a consumer can't tell "inactive" from "absent".)
func TestCrmFillingField_FalseBoolsSurviveJSON(t *testing.T) {
	b, err := json.Marshal(CrmFillingField{UUID: "f1", IsActive: false, IsAutoPush: false})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"isActive", "isAutoPush"} {
		v, ok := got[key]
		if !ok {
			t.Errorf("%s missing from JSON %s", key, b)
			continue
		}
		if v != false {
			t.Errorf("%s = %v, want false", key, v)
		}
	}
}

// --- teams writes + members ---

func TestCreateTeam(t *testing.T) {
	srv, cap := recordingServer(t, 201, `{"id":5,"name":"Sales"}`)
	c := newTestClient(srv.URL)
	team, err := c.CreateTeam(context.Background(), CreateTeamInput{Name: "Sales"})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/teams" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if team.Name != "Sales" {
		t.Fatalf("team = %+v", team)
	}
}

func TestUpdateTeam(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"id":5,"name":"Renamed"}`)
	c := newTestClient(srv.URL)
	team, err := c.UpdateTeam(context.Background(), "5", UpdateTeamInput{Name: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPatch || cap.path != "/teams/5" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if team.Name != "Renamed" {
		t.Fatalf("team = %+v", team)
	}
}

func TestDeleteTeam(t *testing.T) {
	srv, cap := recordingServer(t, 204, "")
	c := newTestClient(srv.URL)
	if err := c.DeleteTeam(context.Background(), "5"); err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodDelete || cap.path != "/teams/5" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
}

func TestTeamMembers_Paginated(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"data":[{"id":1,"email":"a@x.com","firstName":"A","lastName":"B"}],"pagination":{"page":1,"size":50,"total":1}}`)
	c := newTestClient(srv.URL)
	var got []TeamMember
	for m, err := range c.TeamMembers(context.Background(), "5", PageFilter{}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, m)
	}
	if cap.path != "/teams/5/members" {
		t.Errorf("path = %s", cap.path)
	}
	if len(got) != 1 || got[0].Email != "a@x.com" {
		t.Fatalf("members = %+v", got)
	}
	// TeamMember aliases User, so firstName/lastName fold into Name exactly as
	// `users get` decodes them — one consistent shape for the same entity.
	if got[0].Name != "A B" {
		t.Errorf("name = %q, want folded \"A B\"", got[0].Name)
	}
}

// --- users update + membership ---

func TestUpdateUser(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"id":7,"email":"u@x.com","jobTitle":"AE"}`)
	c := newTestClient(srv.URL)
	u, err := c.UpdateUser(context.Background(), "7", UpdateUserInput{JobTitle: Ptr("AE")})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPatch || cap.path != "/users/7" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(cap.body), &sent)
	if sent["jobTitle"] != "AE" {
		t.Errorf("jobTitle not sent: %s", cap.body)
	}
	if _, ok := sent["email"]; ok {
		t.Errorf("omitempty: empty email should not be sent: %s", cap.body)
	}
	// The User model folds the API's jobTitle into Title via UnmarshalJSON.
	if u.Title != "AE" {
		t.Fatalf("user = %+v", u)
	}
}

func TestAddUserTeam(t *testing.T) {
	srv, cap := recordingServer(t, 201, `{"userId":7,"teamId":5}`)
	c := newTestClient(srv.URL)
	ut, err := c.AddUserTeam(context.Background(), "7", 5)
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPost || cap.path != "/users/7/teams" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(cap.body), &sent)
	if sent["teamId"] != float64(5) {
		t.Errorf("teamId not sent: %s", cap.body)
	}
	if ut.TeamID.String() != "5" || ut.UserID.String() != "7" {
		t.Errorf("UserTeam = %+v", ut)
	}
}

func TestRemoveUserTeam(t *testing.T) {
	srv, cap := recordingServer(t, 204, "")
	c := newTestClient(srv.URL)
	if err := c.RemoveUserTeam(context.Background(), "7", "5"); err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodDelete || cap.path != "/users/7/teams/5" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
}

// --- webhook update ---

func TestUpdateWebhook(t *testing.T) {
	srv, cap := recordingServer(t, 200, `{"uuid":"wh-1","name":"Renamed","url":"https://x"}`)
	c := newTestClient(srv.URL)
	w, err := c.UpdateWebhook(context.Background(), "wh-1", UpdateWebhookInput{Name: Ptr("Renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if cap.method != http.MethodPatch || cap.path != "/webhooks/wh-1" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if w.Name != "Renamed" {
		t.Fatalf("webhook = %+v", w)
	}
}
