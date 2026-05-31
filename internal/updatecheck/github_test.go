package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatest(t *testing.T) {
	// Happy path: returns the tag_name.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("missing Accept header")
		}
		_, _ = w.Write([]byte(`{"tag_name":"v2.3.4"}`))
	}))
	defer srv.Close()
	got, err := fetchLatest(context.Background(), srv.Client(), srv.URL)
	if err != nil || got != "v2.3.4" {
		t.Fatalf("fetchLatest = %q, %v", got, err)
	}
}

func TestFetchLatestNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // e.g. rate limited
	}))
	defer srv.Close()
	got, err := fetchLatest(context.Background(), srv.Client(), srv.URL)
	if err != nil || got != "" {
		t.Fatalf("non-200 should yield empty, no error: %q %v", got, err)
	}
}

func TestFetchLatestBadBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	if _, err := fetchLatest(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestFetchLatestBadURL(t *testing.T) {
	if _, err := fetchLatest(context.Background(), http.DefaultClient, "://bad"); err == nil {
		t.Fatal("expected request build error")
	}
}

func TestGitHubLatestUnreachable(t *testing.T) {
	// Points at the real (sandbox-blocked) endpoint; must fail gracefully.
	if _, err := GitHubLatest(context.Background()); err == nil {
		t.Skip("network reachable; nothing to assert")
	}
}
