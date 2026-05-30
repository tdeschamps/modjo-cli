package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewDefaultsHTTPClient(t *testing.T) {
	c := New(Options{Endpoint: "http://x"})
	if c.http == nil {
		t.Error("New should default the HTTP client")
	}
}

func TestToolsDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(readBody(r), "initialize") {
			writeResult(w, []byte("1"), []byte(`{}`))
			return
		}
		// tools/list result is not the expected shape.
		writeResult(w, []byte("2"), []byte(`{"tools":"not-an-array"}`))
	}))
	defer srv.Close()
	if _, err := newClient(srv.URL).Tools(context.Background()); err == nil {
		t.Error("expected decode error")
	}
}

func TestRPCDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()
	if _, err := newClient(srv.URL).Tools(context.Background()); err == nil {
		t.Error("expected rpc decode error")
	}
}

func TestExtractTextDecodeError(t *testing.T) {
	if _, _, err := extractText([]byte("not json")); err == nil {
		t.Error("expected extractText decode error")
	}
}

func TestTokenErrorPropagates(t *testing.T) {
	c := New(Options{Endpoint: "http://x", Token: func() (string, error) { return "", context.Canceled }})
	if _, err := c.Tools(context.Background()); err == nil {
		t.Error("token error should propagate through ensureInit")
	}
}

func TestServerToolsError(t *testing.T) {
	// Upstream returns an HTTP error; the server's tools/list handler surfaces it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	server := NewServer(newClient(srv.URL))
	out := &strings.Builder{}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := server.ServeStdio(context.Background(), in, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "error") {
		t.Errorf("server should report upstream error: %s", out.String())
	}
}

func TestServerCallBadParams(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	server := NewServer(newClient(srv.URL))
	out := &strings.Builder{}
	// tools/call with malformed params → decode error in handle.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}` + "\n")
	if err := server.ServeStdio(context.Background(), in, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "error") {
		t.Errorf("expected param decode error: %s", out.String())
	}
}

func TestRPC202ForRequestIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(readBody(r), "initialize") {
			w.Header().Set("Content-Type", "application/json")
			writeResult(w, []byte("1"), []byte(`{}`))
			return
		}
		// A real request gets 202 with no body — must surface as an error,
		// not a nil result that the caller nil-derefs on.
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	c := newClient(srv.URL)
	if _, err := c.Tools(context.Background()); err == nil {
		t.Error("202 for a request should be an error, not an empty result")
	}
}

func TestServerPing(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	server := NewServer(newClient(srv.URL))
	out := &strings.Builder{}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	if err := server.ServeStdio(context.Background(), in, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"result"`) {
		t.Errorf("ping should return a result: %s", out.String())
	}
}
