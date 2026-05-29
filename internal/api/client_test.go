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

// fixtureServer serves two pages of deals; page 2 has no nextCursor.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/deals", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")
		makePage := func(start, n int, next string) listResponse {
			vals := make([]json.RawMessage, n)
			for i := 0; i < n; i++ {
				vals[i] = json.RawMessage(fmt.Sprintf(`{"crmId":"D%d","name":"Deal %d","status":"Open"}`, start+i, start+i))
			}
			return listResponse{Values: vals, Pagination: pagination{NextCursor: next}}
		}
		var resp listResponse
		switch cursor {
		case "":
			resp = makePage(0, 50, "page2")
		case "page2":
			resp = makePage(50, 25, "")
		default:
			t.Errorf("unexpected cursor %q", cursor)
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
	for d, err := range c.Deals(context.Background(), DealFilter{Status: []string{"Open"}}) {
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

	_, err := c.GetDeal(context.Background(), "nope")
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
		Account: "001ABC",
		User:    "u1",
		Since:   "2026-05-01",
		Until:   "2026-05-29",
		Limit:   50,
	}
	q := f.query("")
	if q.Get("accountCrmId") != "001ABC" {
		t.Errorf("accountCrmId = %q", q.Get("accountCrmId"))
	}
	if q.Get("startDate") != "2026-05-01" {
		t.Errorf("startDate = %q", q.Get("startDate"))
	}
	if got, _ := strconv.Atoi(q.Get("limit")); got != 50 {
		t.Errorf("limit = %q", q.Get("limit"))
	}
}
