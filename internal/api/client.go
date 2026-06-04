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

// pagination is the page-based envelope the API returns: 1-indexed page, the
// page size actually served, and the grand total across all pages.
type pagination struct {
	Page  int `json:"page"`
	Size  int `json:"size"`
	Total int `json:"total"`
}

type listResponse struct {
	Data       []json.RawMessage `json:"data"`
	Pagination pagination        `json:"pagination"`
}

// getData fetches a sub-resource that wraps its rows in a {"data":[...]}
// envelope without pagination (e.g. /calls/{id}/transcript, /summaries) and
// decodes them into []T. It keeps the {data} envelope handled in one place
// rather than re-implemented per command.
func getData[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var env struct {
		Data []T `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
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

// queryFunc builds the query for a given 1-indexed page.
type queryFunc func(page int) url.Values

// paginate returns an iterator over a list endpoint, walking pages (page=N,
// 1-indexed) and honoring limit. It decodes each raw value into a fresh T and
// stops once the reported total is covered, a short/empty page arrives, or the
// limit is reached.
func paginate[T any](ctx context.Context, c *Client, path string, q queryFunc, limit int) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		emitted := 0 // rows yielded toward the caller's limit
		fetched := 0 // rows received from the server (for total-coverage stop)
		for page := 1; ; page++ {
			var resp listResponse
			if err := c.doJSON(ctx, http.MethodGet, path, q(page), nil, &resp); err != nil {
				yield(zero, err)
				return
			}
			for _, raw := range resp.Data {
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
			// Stop when the page is empty, when we've fetched every item the
			// server reports (cumulative count — robust to short pages), or when
			// total is unknown and a short page signals the end. `fetched` counts
			// rows actually returned, independent of the limit-based `emitted`.
			fetched += len(resp.Data)
			if len(resp.Data) == 0 {
				return
			}
			if resp.Pagination.Total > 0 && fetched >= resp.Pagination.Total {
				return
			}
			if resp.Pagination.Total == 0 && len(resp.Data) < pageSize {
				return
			}
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

// Users lists users.
func (c *Client) Users(ctx context.Context, f UserFilter) iter.Seq2[User, error] {
	return paginate[User](ctx, c, "/users", f.query, f.Limit)
}

// Teams lists teams.
func (c *Client) Teams(ctx context.Context, f TeamFilter) iter.Seq2[Team, error] {
	return paginate[Team](ctx, c, "/teams", f.query, f.Limit)
}

// Tags lists call tags.
func (c *Client) Tags(ctx context.Context, f TagFilter) iter.Seq2[Tag, error] {
	return paginate[Tag](ctx, c, "/tags", f.query, f.Limit)
}

// Topics lists conversation topics.
func (c *Client) Topics(ctx context.Context, f TopicFilter) iter.Seq2[Topic, error] {
	return paginate[Topic](ctx, c, "/topics", f.query, f.Limit)
}

// Webhooks lists webhooks.
func (c *Client) Webhooks(ctx context.Context, f WebhookFilter) iter.Seq2[Webhook, error] {
	return paginate[Webhook](ctx, c, "/webhooks", f.query, f.Limit)
}

// --- single-object gets ---

// GetCall fetches one call, optionally expanding related entities (one of
// contacts, deal, account, users).
func (c *Client) GetCall(ctx context.Context, id string, expand ...string) (Call, error) {
	q := url.Values{}
	for _, e := range expand {
		q.Add("expand", e)
	}
	var out Call
	err := c.doJSON(ctx, http.MethodGet, "/calls/"+url.PathEscape(id), q, nil, &out)
	return out, err
}

// GetCallTranscript fetches a call's transcript blocks (empty while the call is
// still processing).
func (c *Client) GetCallTranscript(ctx context.Context, id string) ([]TranscriptBlock, error) {
	return getData[TranscriptBlock](ctx, c, "/calls/"+url.PathEscape(id)+"/transcript")
}

// GetCallSummaries fetches a call's pre-generated summaries.
func (c *Client) GetCallSummaries(ctx context.Context, id string) ([]CallSummary, error) {
	return getData[CallSummary](ctx, c, "/calls/"+url.PathEscape(id)+"/summaries")
}

// GetDeal fetches one deal by numeric id. Note: the spec exposes
// GET /deals/{id}/summary but no plain GET /deals/{id}; this is kept for
// callers that resolve a deal from the list endpoint.
func (c *Client) GetDeal(ctx context.Context, id string) (Deal, error) {
	var out Deal
	err := c.doJSON(ctx, http.MethodGet, "/deals/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetAccount fetches one account by numeric id.
func (c *Client) GetAccount(ctx context.Context, id string) (Account, error) {
	var out Account
	err := c.doJSON(ctx, http.MethodGet, "/accounts/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetContact fetches one contact by numeric id.
func (c *Client) GetContact(ctx context.Context, id string) (Contact, error) {
	var out Contact
	err := c.doJSON(ctx, http.MethodGet, "/contacts/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetUser fetches one user by numeric id.
func (c *Client) GetUser(ctx context.Context, id string) (User, error) {
	var out User
	err := c.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetTeam fetches one team by numeric id.
func (c *Client) GetTeam(ctx context.Context, id string) (Team, error) {
	var out Team
	err := c.doJSON(ctx, http.MethodGet, "/teams/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// GetWebhook fetches one webhook by uuid.
func (c *Client) GetWebhook(ctx context.Context, uuid string) (Webhook, error) {
	var out Webhook
	err := c.doJSON(ctx, http.MethodGet, "/webhooks/"+url.PathEscape(uuid), nil, nil, &out)
	return out, err
}

// --- writes ---

// CreateUserInput is the payload for creating a user (OpenAPI
// CreateUserRequest). email, firstName and lastName are required; the rest are
// optional.
type CreateUserInput struct {
	Email         string `json:"email"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	PhoneNumber   string `json:"phoneNumber,omitempty"`
	JobTitle      string `json:"jobTitle,omitempty"`
	JobDepartment string `json:"jobDepartment,omitempty"`
	Role          string `json:"role,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
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

// CreateWebhookInput is the payload for creating a webhook (OpenAPI
// CreateWebhookRequest). All fields are required.
type CreateWebhookInput struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// CreateWebhook creates a webhook (POST /webhooks -> 201 Webhook).
func (c *Client) CreateWebhook(ctx context.Context, in CreateWebhookInput) (Webhook, error) {
	body, _ := json.Marshal(in)
	var out Webhook
	err := c.doJSON(ctx, http.MethodPost, "/webhooks", nil, bytes.NewReader(body), &out)
	return out, err
}

// DeleteWebhook deletes a webhook by uuid (DELETE /webhooks/{uuid} -> 204).
func (c *Client) DeleteWebhook(ctx context.Context, uuid string) error {
	return c.doJSON(ctx, http.MethodDelete, "/webhooks/"+url.PathEscape(uuid), nil, nil, nil)
}

// Me validates the active credential against a lightweight authed endpoint and
// returns its raw response. The API has no dedicated identity route, so we use
// a minimal read of /users (page=1): it requires a valid key (401 otherwise),
// stays cheap, and exists on every workspace. Callers use it only to confirm
// reachability and auth, not to read a specific principal.
func (c *Client) Me(ctx context.Context) (json.RawMessage, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users", url.Values{"page": {"1"}}, nil)
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
