// Package mcp is the Model Context Protocol adapter. The Client speaks JSON-RPC
// 2.0 over the Modjo MCP "Streamable HTTP" endpoint and powers `modjo ask` and
// `modjo mcp tools/call`. The same package also hosts the embedded server used
// by `modjo mcp serve`.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// NativeAgents maps the built-in Modjo agent names to their UUIDs so
// `--agent <name>` resolves offline (product spec Appendix B).
var NativeAgents = map[string]string{
	"CallSummary":    "741e9ffc-87be-4bca-bb0d-f167be8b963e",
	"NextStepper":    "09715241-0cdd-44c9-a386-92a1340bdf4a",
	"MeetingPrepper": "c0a76fd7-3f56-4a03-b2e7-7765da10c457",
	"CallQualifier":  "3a7753e1-d21e-4cb7-a990-7820291274cd",
	"DealBriefing":   "1204e84f-6edd-4782-bbdf-e5e070b400cf",
	"EmailFollowUp":  "b2a9ae1b-2026-4dfa-9c67-101733a87a04",
}

// Tool describes an MCP tool exposed by the server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// AskOpts are options for the ask_anything_on_* tools.
type AskOpts struct {
	Agent    string // UUID
	Language string
}

// Answer is the normalized result of an ask.
type Answer struct {
	Answer string          `json:"answer"`
	Agent  string          `json:"agent,omitempty"`
	Entity string          `json:"entity,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

// Options configures a Client.
type Options struct {
	Endpoint   string
	HTTPClient *http.Client
	Token      func() (string, error)
}

// Client is an MCP JSON-RPC client.
type Client struct {
	endpoint string
	http     *http.Client
	token    func() (string, error)

	mu        sync.Mutex
	idCounter int
	sessionID string
	inited    bool
}

// New constructs an MCP Client.
func New(opt Options) *Client {
	hc := opt.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{endpoint: opt.Endpoint, http: hc, token: opt.Token}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message) }

func (c *Client) nextID() json.RawMessage {
	c.idCounter++
	return json.RawMessage(fmt.Sprintf("%d", c.idCounter))
}

// ensureInit performs the MCP initialize handshake once per client.
func (c *Client) ensureInit(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inited {
		return nil
	}
	params, _ := json.Marshal(map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "modjo-cli", "version": "dev"},
	})
	if _, err := c.rpc(ctx, "initialize", params); err != nil {
		return err
	}
	// Best-effort initialized notification (no response expected).
	_ = c.notify(ctx, "notifications/initialized")
	c.inited = true
	return nil
}

// rpc sends a JSON-RPC request and returns the result payload.
func (c *Client) rpc(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	reqBody, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: c.nextID(), Method: method, Params: params})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	if c.token != nil {
		tok, err := c.token()
		if err != nil {
			return nil, err
		}
		if tok != "" {
			httpReq.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	payload := data
	// Streamable HTTP may return SSE; extract the JSON-RPC data line(s).
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		payload = extractSSEData(data)
	}
	var rpcResp rpcResponse
	if err := json.Unmarshal(payload, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode MCP response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

// notify sends a notification (no id, no response body expected).
func (c *Client) notify(ctx context.Context, method string) error {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	if c.token != nil {
		if tok, err := c.token(); err == nil && tok != "" {
			httpReq.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

// Tools lists the tools the upstream MCP server exposes.
func (c *Client) Tools(ctx context.Context) ([]Tool, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	res, err := c.rpc(ctx, "tools/list", json.RawMessage(`{}`))
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

// Call invokes a tool and returns the raw result payload.
func (c *Client) Call(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	return c.rpc(ctx, "tools/call", params)
}

// toolResult is the standard MCP tools/call result envelope.
type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// extractText concatenates text content blocks from a tools/call result.
func extractText(raw json.RawMessage) (string, *toolResult, error) {
	var tr toolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", nil, err
	}
	var b strings.Builder
	for _, c := range tr.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String(), &tr, nil
}

func (c *Client) ask(ctx context.Context, tool, idKey, idVal, question string, opt AskOpts) (Answer, error) {
	args := map[string]any{idKey: idVal, "question": question}
	if opt.Agent != "" {
		args["agentId"] = opt.Agent
	}
	if opt.Language != "" {
		args["language"] = opt.Language
	}
	raw, err := c.Call(ctx, tool, args)
	if err != nil {
		return Answer{}, err
	}
	text, tr, err := extractText(raw)
	if err != nil {
		return Answer{}, err
	}
	if tr.IsError {
		return Answer{}, fmt.Errorf("ask failed: %s", text)
	}
	return Answer{Answer: text, Agent: opt.Agent, Entity: idVal, Raw: raw}, nil
}

// AskOnCall asks a natural-language question about a call.
func (c *Client) AskOnCall(ctx context.Context, callID, question string, opt AskOpts) (Answer, error) {
	return c.ask(ctx, "ask_anything_on_call", "callId", callID, question, opt)
}

// AskOnDeal asks a natural-language question about a deal.
func (c *Client) AskOnDeal(ctx context.Context, crmID, question string, opt AskOpts) (Answer, error) {
	return c.ask(ctx, "ask_anything_on_deal", "dealCrmId", crmID, question, opt)
}

// AskOnAccount asks a natural-language question about an account.
func (c *Client) AskOnAccount(ctx context.Context, crmID, question string, opt AskOpts) (Answer, error) {
	return c.ask(ctx, "ask_anything_on_account", "accountCrmId", crmID, question, opt)
}

// extractSSEData pulls the concatenated `data:` lines out of an SSE payload.
func extractSSEData(b []byte) []byte {
	var out bytes.Buffer
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "data:") {
			out.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if out.Len() == 0 {
		return b
	}
	return out.Bytes()
}
