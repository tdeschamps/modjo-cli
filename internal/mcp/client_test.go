package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubMCP replies to JSON-RPC requests for initialize, tools/list, tools/call.
func stubMCP(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("auth header = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req rpcRequest
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sess-1")

		switch req.Method {
		case "initialize":
			writeResult(w, req.ID, json.RawMessage(`{"protocolVersion":"2025-06-18","serverInfo":{"name":"modjo"}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			writeResult(w, req.ID, json.RawMessage(`{"tools":[{"name":"ask_anything_on_call","description":"Ask about a call"},{"name":"get_deals","description":"List deals"}]}`))
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.Name != "ask_anything_on_call" {
				t.Errorf("tool name = %q", p.Name)
			}
			writeResult(w, req.ID, json.RawMessage(`{"content":[{"type":"text","text":"They raised pricing objections."}]}`))
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
}

func writeResult(w http.ResponseWriter, id json.RawMessage, result json.RawMessage) {
	resp := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	_ = json.NewEncoder(w).Encode(resp)
}

func newClient(url string) *Client {
	return New(Options{Endpoint: url, HTTPClient: http.DefaultClient, Token: func() (string, error) { return "key", nil }})
}

func TestTools(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	c := newClient(srv.URL)
	tools, err := c.Tools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "ask_anything_on_call" {
		t.Fatalf("tools = %+v", tools)
	}
}

func TestAskOnCall(t *testing.T) {
	srv := stubMCP(t)
	defer srv.Close()
	c := newClient(srv.URL)
	ans, err := c.AskOnCall(context.Background(), "74969", "What were the objections?", AskOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Answer, "pricing objections") {
		t.Errorf("answer = %q", ans.Answer)
	}
}

func TestErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req rpcRequest
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			writeResult(w, req.ID, json.RawMessage(`{}`))
			return
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32602, "message": "unknown tool"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	c := New(Options{Endpoint: srv.URL, HTTPClient: http.DefaultClient})
	_, err := c.Call(context.Background(), "nope", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("want rpc error, got %v", err)
	}
}
