package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestServeStdio(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	upstream := newClient(srv.URL)
	server := NewServer(upstream)

	// A sequence of framed JSON-RPC messages: initialize, a notification
	// (no id, no response), tools/list, tools/call, and an unknown method.
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ask_anything_on_call","arguments":{"callId":"1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"bogus/method"}`,
	}, "\n") + "\n"

	var out strings.Builder
	if err := server.ServeStdio(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	got := out.String()
	for _, want := range []string{`"protocolVersion"`, `"tools"`, `pricing objections`, `method not found`} {
		if !strings.Contains(got, want) {
			t.Errorf("ServeStdio output missing %q:\n%s", want, got)
		}
	}
	// The notification must not produce a response line.
	if strings.Count(got, `"id":`) != 4 {
		t.Errorf("expected 4 responses (notification suppressed), got: %s", got)
	}
}

func TestServeStdioCancelled(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	server := NewServer(newClient(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// With a cancelled context and a non-empty stream, the loop returns the
	// context error before processing.
	err := server.ServeStdio(ctx, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), &strings.Builder{})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestExtractSSEData(t *testing.T) {
	sse := "event: message\ndata: {\"a\":1}\n\n"
	got := extractSSEData([]byte(sse))
	if strings.TrimSpace(string(got)) != `{"a":1}` {
		t.Errorf("extractSSEData = %q", got)
	}
	// No data lines → returns input unchanged.
	plain := []byte(`{"b":2}`)
	if string(extractSSEData(plain)) != `{"b":2}` {
		t.Errorf("passthrough failed: %q", extractSSEData(plain))
	}
}
