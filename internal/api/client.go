// Package api is the REST v2 adapter: typed models, filters, and a Client whose
// list methods return auto-paginating iterators (iter.Seq2). It sits behind the
// ModjoAPI interface so services and commands depend on the seam, not the wire.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"
)

// Options configures a Client.
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	// Token is used only when HTTPClient does not already inject auth (the
	// httpclient chain normally handles it). Kept for direct/test usage.
	Token func() (string, error)
}

// Client talks to the Modjo REST API v2.
type Client struct {
	baseURL string
	http    *http.Client
	token   func() (string, error)
}

// New constructs a Client.
func New(opt Options) *Client {
	hc := opt.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		baseURL: strings.TrimRight(opt.BaseURL, "/"),
		http:    hc,
		token:   opt.Token,
	}
}

type pagination struct {
	NextCursor string `json:"nextCursor"`
}

type listResponse struct {
	Values     []json.RawMessage `json:"values"`
	Pagination pagination        `json:"pagination"`
}

// doJSON performs a request and decodes a JSON response into out. It maps
// non-2xx responses to *Error.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	raw, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) ([]byte, error) {
	u := c.baseURL + ensureLeadingSlash(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// If the http.Client doesn't inject auth, do it here.
	if c.token != nil && req.Header.Get("Authorization") == "" {
		tok, err := c.token()
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAPIError(resp.StatusCode, resp.Header.Get("X-Request-Id"), data)
	}
	return data, nil
}

// Raw performs an arbitrary authenticated request (used by `modjo api`).
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	return c.do(ctx, method, path, query, r)
}

// queryFunc builds the query for a given cursor.
type queryFunc func(cursor string) url.Values

// paginate returns an iterator over a list endpoint, following nextCursor and
// honoring limit. It decodes each raw value into a fresh T.
func paginate[T any](ctx context.Context, c *Client, path string, q queryFunc, limit int) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		cursor := ""
		emitted := 0
		for {
			var resp listResponse
			if err := c.doJSON(ctx, http.MethodGet, path, q(cursor), nil, &resp); err != nil {
				yield(zero, err)
				return
			}
			for _, raw := range resp.Values {
				var item T
				if err := json.Unmarshal(raw, &item); err != nil {
					if !yield(zero, fmt.Errorf("decode %s item: %w", path, err)) {
						return
					}
				} else if !yield(item, nil) {
					return
				}
				// Count every yield (success or decode error) toward the limit
				// so a fully-malformed page can't make us page the whole dataset.
				emitted++
				if limit > 0 && emitted >= limit {
					return
				}
			}
			if resp.Pagination.NextCursor == "" {
				return
			}
			cursor = resp.Pagination.NextCursor
		}
	}
}

// --- typed list endpoints ---

// Calls lists calls.
func (c *Client) Calls(ctx context.Context, f CallFilter) iter.Seq2[Call, error] {
	return paginate[Call](ctx, c, "/calls", f.query, f.Limit)
}

// Deals lists deals.
func (c *Client) Deals(ctx context.Context, f DealFilter) iter.Seq2[Deal, error] {
	return paginate[Deal](ctx, c, "/deals", f.query, f.Limit)
}

// Accounts lists accounts.
func (c *Client) Accounts(ctx context.Context, f AccountFilter) iter.Seq2[Account, error] {
	return paginate[Account](ctx, c, "/accounts", f.query, f.Limit)
}

// Contacts lists contacts.
func (c *Client) Contacts(ctx context.Context, f ContactFilter) iter.Seq2[Contact, error] {
	return paginate[Contact](ctx, c, "/contacts", f.query, f.Limit)
}

// Emails lists emails.
func (c *Client) Emails(ctx context.Context, f EmailFilter) iter.Seq2[Email, error] {
	return paginate[Email](ctx, c, "/emails", f.query, f.Limit)
}

// Users lists users.
func (c *Client) Users(ctx context.Context, f UserFilter) iter.Seq2[User, error] {
	return paginate[User](ctx, c, "/users", f.query, f.Limit)
}

// Teams lists teams.
func (c *Client) Teams(ctx context.Context) iter.Seq2[Team, error] {
	return paginate[Team](ctx, c, "/teams", func(cursor string) url.Values {
		q := url.Values{}
		setPaging(q, 0, cursor)
		return q
	}, 0)
}

// Agents lists agents.
func (c *Client) Agents(ctx context.Context, f AgentFilter) iter.Seq2[Agent, error] {
	return paginate[Agent](ctx, c, "/agents", f.query, f.Limit)
}

// --- single-object gets ---

// GetCall fetches one call, optionally with relations.
func (c *Client) GetCall(ctx context.Context, id string, relations ...string) (Call, error) {
	q := url.Values{}
	if len(relations) > 0 {
		q.Set("relations", strings.Join(relations, ","))
	}
	var out Call
	err := c.doJSON(ctx, http.MethodGet, "/calls/"+url.PathEscape(id), q, nil, &out)
	return out, err
}

// GetDeal fetches one deal.
func (c *Client) GetDeal(ctx context.Context, crmID string) (Deal, error) {
	var out Deal
	err := c.doJSON(ctx, http.MethodGet, "/deals/"+url.PathEscape(crmID), nil, nil, &out)
	return out, err
}

// GetAccount fetches one account.
func (c *Client) GetAccount(ctx context.Context, crmID string) (Account, error) {
	var out Account
	err := c.doJSON(ctx, http.MethodGet, "/accounts/"+url.PathEscape(crmID), nil, nil, &out)
	return out, err
}

// GetContact fetches one contact.
func (c *Client) GetContact(ctx context.Context, crmPersonID string) (Contact, error) {
	var out Contact
	err := c.doJSON(ctx, http.MethodGet, "/contacts/"+url.PathEscape(crmPersonID), nil, nil, &out)
	return out, err
}

// GetEmail fetches one email (including content).
func (c *Client) GetEmail(ctx context.Context, id string) (Email, error) {
	var out Email
	err := c.doJSON(ctx, http.MethodGet, "/emails/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetUser fetches one user.
func (c *Client) GetUser(ctx context.Context, id string) (User, error) {
	var out User
	err := c.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetTeam fetches one team.
func (c *Client) GetTeam(ctx context.Context, id string) (Team, error) {
	var out Team
	err := c.doJSON(ctx, http.MethodGet, "/teams/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetAgent fetches one agent.
func (c *Client) GetAgent(ctx context.Context, uuid string) (Agent, error) {
	var out Agent
	err := c.doJSON(ctx, http.MethodGet, "/agents/"+url.PathEscape(uuid), nil, nil, &out)
	return out, err
}

// --- writes ---

// CreateUserInput is the payload for creating a user.
type CreateUserInput struct {
	Email  string `json:"email"`
	Role   string `json:"role,omitempty"`
	TeamID string `json:"teamId,omitempty"`
}

// CreateUser creates a user (REST-only; the MCP is read-only).
func (c *Client) CreateUser(ctx context.Context, in CreateUserInput) (User, error) {
	body, _ := json.Marshal(in)
	var out User
	err := c.doJSON(ctx, http.MethodPost, "/users", nil, bytes.NewReader(body), &out)
	return out, err
}

// DeleteUser deletes a user by ID.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/users/"+url.PathEscape(id), nil, nil, nil)
}

// Me returns the authenticated principal, used to validate credentials.
func (c *Client) Me(ctx context.Context) (json.RawMessage, error) {
	raw, err := c.do(ctx, http.MethodGet, "/me", nil, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
