package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// allEndpointStub answers every typed endpoint with a minimal valid body.
func allEndpointStub(t *testing.T) *Client {
	t.Helper()
	mux := http.NewServeMux()
	one := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }
	}
	page := `{"data":[{}],"pagination":{"page":1,"size":50,"total":1}}`
	mux.HandleFunc("/calls", one(page))
	mux.HandleFunc("/calls/", one(`{"id":1,"name":"t"}`))
	mux.HandleFunc("/deals", one(page))
	mux.HandleFunc("/deals/", one(`{"id":1,"name":"d"}`))
	mux.HandleFunc("/accounts", one(page))
	mux.HandleFunc("/accounts/", one(`{"id":1,"name":"a"}`))
	mux.HandleFunc("/contacts", one(page))
	mux.HandleFunc("/contacts/", one(`{"id":1,"name":"c"}`))
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
	mux.HandleFunc("/tags", one(page))
	mux.HandleFunc("/topics", one(page))
	mux.HandleFunc("/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"uuid":"wh-1","name":"n","url":"https://x"}`))
			return
		}
		_, _ = w.Write([]byte(page))
	})
	mux.HandleFunc("/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"uuid":"wh-1","name":"n","url":"https://x"}`))
	})
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
	if _, err := c.GetAccount(ctx, "1"); err != nil {
		t.Errorf("GetAccount: %v", err)
	}
	if _, err := c.GetContact(ctx, "1"); err != nil {
		t.Errorf("GetContact: %v", err)
	}
	if _, err := c.GetUser(ctx, "1"); err != nil {
		t.Errorf("GetUser: %v", err)
	}
	if _, err := c.GetTeam(ctx, "1"); err != nil {
		t.Errorf("GetTeam: %v", err)
	}
	if _, err := c.GetWebhook(ctx, "wh-1"); err != nil {
		t.Errorf("GetWebhook: %v", err)
	}
}

func TestAllListers(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if err := drain(c.Calls(ctx, CallFilter{Account: "1", Deal: "2", User: "3", Since: "2026-01-01", Until: "2026-02-01", Expand: []string{"transcript"}})); err != nil {
		t.Errorf("Calls: %v", err)
	}
	if err := drain(c.Deals(ctx, DealFilter{Status: "open", Account: "1", Name: "x"})); err != nil {
		t.Errorf("Deals: %v", err)
	}
	if err := drain(c.Accounts(ctx, AccountFilter{Name: "x"})); err != nil {
		t.Errorf("Accounts: %v", err)
	}
	if err := drain(c.Contacts(ctx, ContactFilter{Name: "x"})); err != nil {
		t.Errorf("Contacts: %v", err)
	}
	if err := drain(c.Users(ctx, UserFilter{Email: "e@x.com"})); err != nil {
		t.Errorf("Users: %v", err)
	}
	if err := drain(c.Teams(ctx, TeamFilter{Name: "t"})); err != nil {
		t.Errorf("Teams: %v", err)
	}
	if err := drain(c.Tags(ctx, TagFilter{})); err != nil {
		t.Errorf("Tags: %v", err)
	}
	if err := drain(c.Topics(ctx, TopicFilter{})); err != nil {
		t.Errorf("Topics: %v", err)
	}
	if err := drain(c.Webhooks(ctx, WebhookFilter{})); err != nil {
		t.Errorf("Webhooks: %v", err)
	}
}

func TestWrites(t *testing.T) {
	c := allEndpointStub(t)
	ctx := context.Background()
	if _, err := c.CreateUser(ctx, CreateUserInput{Email: "new@x.com", FirstName: "New", LastName: "User", Role: "rep"}); err != nil {
		t.Errorf("CreateUser: %v", err)
	}
	if err := c.DeleteUser(ctx, "1"); err != nil {
		t.Errorf("DeleteUser: %v", err)
	}
	if _, err := c.CreateWebhook(ctx, CreateWebhookInput{Name: "n", URL: "https://x", Events: []string{"call_summarized"}}); err != nil {
		t.Errorf("CreateWebhook: %v", err)
	}
	if err := c.DeleteWebhook(ctx, "wh-1"); err != nil {
		t.Errorf("DeleteWebhook: %v", err)
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

func TestPaginateDecodeErrorsCountTowardLimit(t *testing.T) {
	// Every page is full of malformed items; with a limit the iterator must
	// stop after `limit` yields instead of paging the whole dataset.
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		_, _ = w.Write([]byte(`{"data":["bad","bad","bad","bad","bad"],"pagination":{"page":1,"size":5,"total":100}}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})

	yields := 0
	for _, err := range c.Deals(context.Background(), DealFilter{Limit: 3}) {
		_ = err
		yields++
		if yields > 100 {
			t.Fatal("iterator did not respect the limit on a corrupt page")
		}
	}
	if yields != 3 {
		t.Errorf("expected 3 yields (limit), got %d", yields)
	}
	if pages != 1 {
		t.Errorf("should not have paged past the first page, fetched %d pages", pages)
	}
}

func TestPaginateDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":["not-an-object"],"pagination":{"page":1,"size":50,"total":1}}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})
	err := drain(c.Deals(context.Background(), DealFilter{}))
	if err == nil {
		t.Error("expected decode error for malformed item")
	}
}

// fullPageDealJSON renders one full page (pageSize rows) with total:0 — the
// misbehaving-server shape that defeats every content-based stop condition.
func fullPageDealJSON() string {
	rows := make([]string, pageSize)
	for i := range rows {
		rows[i] = `{"crmId":"D","name":"x"}`
	}
	return `{"data":[` + strings.Join(rows, ",") + `],"pagination":{"page":1,"size":50,"total":0}}`
}

func TestPaginateStopsAtMaxPagesOnRunawayServer(t *testing.T) {
	// A server that ignores `page` and always returns a full page with total:0
	// satisfies no content-based stop (never empty, total unknown, never short).
	// Under --all (limit 0) the iterator must terminate via the page ceiling and
	// surface an error, not loop forever. The handler counts requests so a buggy
	// (unbounded) implementation trips the guard instead of hanging the test.
	var requests int32
	body := fullPageDealJSON()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) > maxPages+5 {
			t.Errorf("paginate did not stop: %d requests (cap is %d)", requests, maxPages)
			http.Error(w, "runaway", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})

	var lastErr error
	rows := 0
	for _, err := range c.Deals(context.Background(), DealFilter{}) { // limit 0 = --all
		if err != nil {
			lastErr = err
			break
		}
		rows++
	}
	if lastErr == nil {
		t.Fatal("expected a page-ceiling error on a runaway server, got nil")
	}
	if got := int(atomic.LoadInt32(&requests)); got > maxPages {
		t.Errorf("fetched %d pages, want <= cap %d", got, maxPages)
	}
}

func TestPaginateLimitStopsBeforeMaxPages(t *testing.T) {
	// The ceiling must not interfere with a normal limited read: even against the
	// runaway server, a small --limit stops cleanly with no error.
	body := fullPageDealJSON()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: func() (string, error) { return "k", nil }})

	rows := 0
	for _, err := range c.Deals(context.Background(), DealFilter{Limit: 10}) {
		if err != nil {
			t.Fatalf("limited read should not error: %v", err)
		}
		rows++
	}
	if rows != 10 {
		t.Errorf("want 10 rows, got %d", rows)
	}
}
