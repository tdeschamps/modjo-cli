package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// fixtureServer serves two pages of deals (50 + 25 of a 75 total) using the
// API's page-based envelope: {"data":[...],"pagination":{page,size,total}}.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/deals", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		makePage := func(start, n, pg int) listResponse {
			vals := make([]json.RawMessage, n)
			for i := 0; i < n; i++ {
				vals[i] = json.RawMessage(fmt.Sprintf(`{"crmId":"D%d","name":"Deal %d","status":"Open"}`, start+i, start+i))
			}
			return listResponse{Data: vals, Pagination: pagination{Page: pg, Size: 50, Total: 75}}
		}
		var resp listResponse
		switch page {
		case 0, 1:
			resp = makePage(0, 50, 1)
		case 2:
			resp = makePage(50, 25, 2)
		default:
			t.Errorf("unexpected page %d", page)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func newTestClient(baseURL string) *Client {
	return New(Options{
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
		Token:      func() (string, error) { return "test-token", nil },
	})
}

func TestDealsList_FollowsPagination(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(srv.URL)

	var got []Deal
	for d, err := range c.Deals(context.Background(), DealFilter{Status: "Open"}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, d)
	}
	if len(got) != 75 {
		t.Fatalf("want 75 deals across 2 pages, got %d", len(got))
	}
	if got[0].CRMID != "D0" || got[74].CRMID != "D74" {
		t.Errorf("unexpected ordering: first=%q last=%q", got[0].CRMID, got[74].CRMID)
	}
}

func TestPaginate_ShortPageWithZeroSizeStillCoversTotal(t *testing.T) {
	// Regression: the old stop test was `page*size >= total`. If the server
	// reports Size=0 on pages, that formula never reaches Total and the loop
	// would spin (or stop wrong). The cumulative-count stop must fetch all rows.
	mux := http.NewServeMux()
	mux.HandleFunc("/deals", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		n := 50
		if page >= 2 {
			n = 20 // short final page; 50 + 20 = 70 total
		}
		vals := make([]json.RawMessage, n)
		for i := range vals {
			vals[i] = json.RawMessage(`{"crmId":"D","name":"x"}`)
		}
		// Size deliberately 0 to defeat any page*size formula.
		_ = json.NewEncoder(w).Encode(listResponse{Data: vals, Pagination: pagination{Page: page, Size: 0, Total: 70}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := newTestClient(srv.URL)

	count := 0
	for _, err := range c.Deals(context.Background(), DealFilter{}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count > 200 {
			t.Fatal("pagination did not terminate")
		}
	}
	if count != 70 {
		t.Errorf("want 70 rows across a short page with Size=0, got %d", count)
	}
}

func TestPaginate_ServedSizeBelowConstantStillPages(t *testing.T) {
	// Regression: when Total is unknown (0), the short-page stop must compare the
	// page length to the size the server actually served, not the local pageSize
	// constant (50). A server paging at size 25 returns full pages of 25; the old
	// `len(Data) < pageSize` check treated every full page as short and stopped
	// after page 1 — silent truncation.
	mux := http.NewServeMux()
	mux.HandleFunc("/deals", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		n := 25 // full page at the server's size
		if page >= 3 {
			n = 0 // end after 3 pages: 25 + 25 + 0
		}
		vals := make([]json.RawMessage, n)
		for i := range vals {
			vals[i] = json.RawMessage(`{"crmId":"D","name":"x"}`)
		}
		// Size=25 (≠ pageSize 50), Total unknown (0).
		_ = json.NewEncoder(w).Encode(listResponse{Data: vals, Pagination: pagination{Page: page, Size: 25, Total: 0}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := newTestClient(srv.URL)

	count := 0
	for _, err := range c.Deals(context.Background(), DealFilter{}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count > 200 {
			t.Fatal("pagination did not terminate")
		}
	}
	if count != 50 {
		t.Errorf("want 50 rows across 2 full pages served at size 25, got %d", count)
	}
}

func TestDealsList_RespectsLimit(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(srv.URL)

	count := 0
	for _, err := range c.Deals(context.Background(), DealFilter{Limit: 10}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 10 {
		t.Errorf("limit not respected: got %d want 10", count)
	}
}

func TestErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_123")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"deal not found"}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)

	_, err := c.GetAccount(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *Error
	if !asError(err, &apiErr) {
		t.Fatalf("want *api.Error, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if apiErr.RequestID != "req_123" {
		t.Errorf("request id = %q", apiErr.RequestID)
	}
}

func TestBuildQueryEncodesFilters(t *testing.T) {
	f := CallFilter{
		Account: "1234",
		User:    "u1",
		Since:   "2026-05-01",
		Until:   "2026-05-29",
		Limit:   50,
	}
	q := f.query(1)
	if q.Get("account_id") != "1234" {
		t.Errorf("account_id = %q", q.Get("account_id"))
	}
	if q.Get("from") != "2026-05-01" {
		t.Errorf("from = %q", q.Get("from"))
	}
	if q.Get("to") != "2026-05-29" {
		t.Errorf("to = %q", q.Get("to"))
	}
	if q.Get("user_id") != "u1" {
		t.Errorf("user_id = %q", q.Get("user_id"))
	}
	if got, _ := strconv.Atoi(q.Get("size")); got != 50 {
		t.Errorf("size = %q", q.Get("size"))
	}
	if q.Get("page") != "1" {
		t.Errorf("page = %q", q.Get("page"))
	}
}
