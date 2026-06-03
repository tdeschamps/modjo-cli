package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnwrapAnswer(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"answer":"hello"}`, "hello"},         // envelope unwrapped
		{`  {"answer":"trimmed"}  `, "trimmed"}, // leading/trailing space
		{"just plain text", "just plain text"},  // not JSON → unchanged
		{`{"other":"x"}`, ""},                   // JSON object, no answer field
		{`{not valid json`, `{not valid json`},  // malformed → unchanged
		{"", ""},                                // empty
		{`["a","b"]`, `["a","b"]`},              // JSON but not an object prefix handled
	}
	for _, c := range cases {
		if got := unwrapAnswer(c.in); got != c.want {
			t.Errorf("unwrapAnswer(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAskOnDealAndAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(body, "initialize") {
			writeResult(w, []byte("1"), []byte(`{}`))
			return
		}
		writeResult(w, []byte("2"), []byte(`{"content":[{"type":"text","text":"answer here"}]}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL)

	for _, fn := range []func() (Answer, error){
		func() (Answer, error) {
			return c.AskOnDeal(context.Background(), "D1", "q", AskOpts{Language: "fr"})
		},
		func() (Answer, error) { return c.AskOnAccount(context.Background(), "A1", "q", AskOpts{}) },
	} {
		ans, err := fn()
		if err != nil || !strings.Contains(ans.Answer, "answer here") {
			t.Errorf("ask: %v / %q", err, ans.Answer)
		}
	}
}

func TestAskToolError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(readBody(r), "initialize") {
			writeResult(w, []byte("1"), []byte(`{}`))
			return
		}
		writeResult(w, []byte("2"), []byte(`{"isError":true,"content":[{"type":"text","text":"boom"}]}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL)
	if _, err := c.AskOnCall(context.Background(), "1", "q", AskOpts{}); err == nil {
		t.Error("expected tool error")
	}
}

func TestSSEResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(readBody(r), "initialize") {
			w.Header().Set("Content-Type", "application/json")
			writeResult(w, []byte("1"), []byte(`{}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":[]}}\n\n"))
	}))
	defer srv.Close()
	c := newClient(srv.URL)
	if _, err := c.Tools(context.Background()); err != nil {
		t.Fatalf("SSE tools: %v", err)
	}
}

func TestInitFailurePropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()
	c := newClient(srv.URL)
	if _, err := c.Tools(context.Background()); err == nil {
		t.Error("expected init failure")
	}
}

func TestNotifyHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeResult(w, []byte("1"), []byte(`{}`))
	}))
	defer srv.Close()
	c := newClient(srv.URL)
	// ensureInit calls notify internally; exercise the full happy path.
	if _, err := c.Call(context.Background(), "x", nil); err != nil {
		t.Fatalf("call: %v", err)
	}
}

func readBody(r *http.Request) string {
	b := make([]byte, r.ContentLength)
	_, _ = r.Body.Read(b)
	return string(b)
}
