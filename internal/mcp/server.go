package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Server is a minimal MCP server (JSON-RPC 2.0 over stdio) that re-exposes the
// upstream Modjo tools. Because it authenticates upstream with the user's
// stored credential, MCP clients (Claude Desktop, Cursor, Codex) never handle
// the raw key. It is built on the same Client, so there is no logic
// duplication.
type Server struct {
	upstream *Client
	name     string
	version  string
}

// NewServer returns a Server proxying to upstream.
func NewServer(upstream *Client) *Server {
	return &Server{upstream: upstream, name: "modjo-cli", version: "dev"}
}

// ServeStdio runs the JSON-RPC loop reading from in and writing to out until in
// is closed or ctx is canceled.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(bufio.NewReader(in))
	enc := json.NewEncoder(out)
	var writeMu sync.Mutex

	send := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return enc.Encode(v)
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// Notifications carry no id and expect no response.
		isNotification := len(req.ID) == 0

		result, rerr := s.handle(ctx, req)
		if isNotification {
			continue
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
		if rerr != nil {
			resp["error"] = map[string]any{"code": -32000, "message": rerr.Error()}
		} else {
			resp["result"] = result
		}
		if err := send(resp); err != nil {
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, req rpcRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.name, "version": s.version},
		}, nil
	case "notifications/initialized", "ping":
		return map[string]any{}, nil
	case "tools/list":
		tools, err := s.upstream.Tools(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		raw, err := s.upstream.Call(ctx, p.Name, p.Arguments)
		if err != nil {
			return nil, err
		}
		// raw is already a tools/call result envelope; pass it through.
		return json.RawMessage(raw), nil
	default:
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}
