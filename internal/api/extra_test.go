package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// allEndpointStub answers every typed endpoint with a minimal valid body.
func allEndpointStub(t *testing.T) *Client {
	t.Helper()
	mux := http.NewServeMux()
	one := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }
	}
	page := `{"values":[{}],"pagination":{"nextCursor":""}}`
	mux.HandleFunc("/me", one(`{"email":"me"}`))
	mux.HandleFunc("/calls", one(page))
	mux.HandleFunc("/calls/", one(`{"id":1,"title":"t"}`))
	mux.HandleFunc("/deals", one(page))
	mux.HandleFunc("/deals/", one(`{"crmId":"D1"}`))
	mux.HandleFunc("/accounts", one(page))
	mux.HandleFunc("/accounts/", one(`{"crmId":"A1"}`))
	mux.HandleFunc("/contacts", one(page))
	mux.HandleFunc("/contacts/", one(`{"crmPersonId":"P1"}`))
	mux.HandleFunc("/emails", one(page))
	mux.HandleFunc("/emails/", one(`{"id":1}`))
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":2,"email":"new"}`))
			return
		}
		_, _ = w.Write([]byte(page))
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"id":1}`))
	})
	mux.HandleFunc("/teams", one(page))
	mux.HandleFunc("/teams/", one(`{"id":1}`))
	mux.HandleFunc("/agents", one(page))
	mux.HandleFunc("/agents/", one(`{"uuid":"u"}`))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})
}

func drain[T any](seq func(func(T, error) bool)) error {
	var err error
	seq(func(_ T, e error) bool {
		if e != nil {
			err = e
			return false
		}
		return true
	})
	return err
}

func TestAllGetters(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if _, err := c.Me(ctx); err != nil {
		t.Errorf("Me: %v", err)
	}
	if _, err := c.GetCall(ctx, "1", "transcript"); err != nil {
		t.Errorf("GetCall: %v", err)
	}
	if _, err := c.GetDeal(ctx, "D1"); err != nil {
		t.Errorf("GetDeal: %v", err)
	}
	if _, err := c.GetAccount(ctx, "A1"); err != nil {
		t.Errorf("GetAccount: %v", err)
	}
	if _, err := c.GetContact(ctx, "P1"); err != nil {
		t.Errorf("GetContact: %v", err)
	}
	if _, err := c.GetEmail(ctx, "1"); err != nil {
		t.Errorf("GetEmail: %v", err)
	}
	if _, err := c.GetUser(ctx, "1"); err != nil {
		t.Errorf("GetUser: %v", err)
	}
	if _, err := c.GetTeam(ctx, "1"); err != nil {
		t.Errorf("GetTeam: %v", err)
	}
	if _, err := c.GetAgent(ctx, "u"); err != nil {
		t.Errorf("GetAgent: %v", err)
	}
}

func TestAllListers(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if err := drain(c.Calls(ctx, CallFilter{Account: "a", Deal: "d", Contact: "c", User: "u", Since: "2026-01-01", Until: "2026-02-01", Relations: []string{"transcript"}})); err != nil {
		t.Errorf("Calls: %v", err)
	}
	if err := drain(c.Deals(ctx, DealFilter{Status: []string{"open"}, Account: "a", CloseBefore: "2026-06-01", CloseAfter: "2026-05-01", AmountMin: 10, AmountMax: 100, Source: []string{"Inbound"}, LossReason: "price"})); err != nil {
		t.Errorf("Deals: %v", err)
	}
	if err := drain(c.Accounts(ctx, AccountFilter{Name: "x"})); err != nil {
		t.Errorf("Accounts: %v", err)
	}
	if err := drain(c.Contacts(ctx, ContactFilter{Name: "x", Account: "a"})); err != nil {
		t.Errorf("Contacts: %v", err)
	}
	if err := drain(c.Emails(ctx, EmailFilter{Account: "a", Deal: "d", Since: "2026-01-01", Until: "2026-02-01"})); err != nil {
		t.Errorf("Emails: %v", err)
	}
	if err := drain(c.Users(ctx, UserFilter{Name: "n", Email: "e", Role: "r", Department: "d"})); err != nil {
		t.Errorf("Users: %v", err)
	}
	if err := drain(c.Teams(ctx)); err != nil {
		t.Errorf("Teams: %v", err)
	}
	if err := drain(c.Agents(ctx, AgentFilter{Search: "s", Origin: "modjo"})); err != nil {
		t.Errorf("Agents: %v", err)
	}
}

func TestWrites(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if _, err := c.CreateUser(ctx, CreateUserInput{Email: "new@x.com", Role: "rep", TeamID: "3"}); err != nil {
		t.Errorf("CreateUser: %v", err)
	}
	if err := c.DeleteUser(ctx, "1"); err != nil {
		t.Errorf("DeleteUser: %v", err)
	}
}

func TestRawAndDecodeError(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if _, err := c.Raw(ctx, http.MethodPost, "/deals", nil, []byte(`{"x":1}`)); err != nil {
		t.Errorf("Raw with body: %v", err)
	}
}

func TestNewDefaultsHTTPClient(t *testing.T) {
	c := New(Options{BaseURL: "http://x"})
	if c.http == nil {
		t.Error("New should default the HTTP client")
	}
}

func TestTokenError(t *testing.T) {
	c := New(Options{BaseURL: "http://x", Token: func() (string, error) { return "", context.Canceled }})
	if _, err := c.Me(context.Background()); err == nil {
		t.Error("token error should propagate")
	}
}

func TestEnsureLeadingSlash(t *testing.T) {
	if ensureLeadingSlash("/x") != "/x" || ensureLeadingSlash("x") != "/x" {
		t.Error("ensureLeadingSlash")
	}
}

func TestPaginateDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":["not-an-object"],"pagination":{}}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})
	err := drain(c.Deals(context.Background(), DealFilter{}))
	if err == nil {
		t.Error("expected decode error for malformed item")
	}
}
