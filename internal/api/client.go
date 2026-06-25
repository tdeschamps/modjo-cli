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

// maxPages caps how many pages paginate will walk in a single sweep. It's a
// safety ceiling, not an expected stop: the normal stops are empty page, total
// coverage, or a short page. A misbehaving server that ignores `page` and keeps
// returning full pages with total:0 satisfies none of those, so without this
// cap an unlimited (`--all`) read would loop forever and buffer rows without
// bound. At pageSize=50 this is 500k rows — far beyond any real workspace — so
// it only ever trips on a server bug, and when it does the caller gets an
// explicit error rather than silent truncation.
const maxPages = 10_000

// paginate returns an iterator over a list endpoint, walking pages (page=N,
// 1-indexed) and honoring limit. It decodes each raw value into a fresh T and
// stops once the reported total is covered, a short/empty page arrives, or the
// limit is reached. As a backstop against a server that never signals the end,
// it stops after maxPages pages and yields an error.
func paginate[T any](ctx context.Context, c *Client, path string, q queryFunc, limit int) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		emitted := 0 // rows yielded toward the caller's limit
		fetched := 0 // rows received from the server (for total-coverage stop)
		for page := 1; ; page++ {
			if page > maxPages {
				yield(zero, fmt.Errorf("paginate %s: stopped after %d pages without an end signal (server may not be honoring pagination)", path, maxPages))
				return
			}
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

// --- call sub-resources ---

// GetCallNotes fetches a call's published notes (GET /calls/{id}/notes).
func (c *Client) GetCallNotes(ctx context.Context, id string) ([]Note, error) {
	return getData[Note](ctx, c, "/calls/"+url.PathEscape(id)+"/notes")
}

// GetCallNextSteps fetches a call's AI-extracted next steps
// (GET /calls/{id}/next-steps). Empty while the call is still processing.
func (c *Client) GetCallNextSteps(ctx context.Context, id string) ([]NextStepItem, error) {
	return getData[NextStepItem](ctx, c, "/calls/"+url.PathEscape(id)+"/next-steps")
}

// GetCallTags lists the tags associated with a call (GET /calls/{id}/tags).
// The id may be an integer id or a uuid.
func (c *Client) GetCallTags(ctx context.Context, id string) ([]Tag, error) {
	return getData[Tag](ctx, c, "/calls/"+url.PathEscape(id)+"/tags")
}

// AddCallTag associates a tag with a call (POST /calls/{id}/tags -> 201
// CallTag). Idempotent: re-adding returns the existing association.
func (c *Client) AddCallTag(ctx context.Context, id string, tagID int) (CallTag, error) {
	body, _ := json.Marshal(addTagToCallInput{TagID: tagID})
	var out CallTag
	err := c.doJSON(ctx, http.MethodPost, "/calls/"+url.PathEscape(id)+"/tags", nil, bytes.NewReader(body), &out)
	return out, err
}

// RemoveCallTag removes a tag from a call (DELETE /calls/{id}/tags/{tagId} ->
// 204).
func (c *Client) RemoveCallTag(ctx context.Context, id, tagID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/calls/"+url.PathEscape(id)+"/tags/"+url.PathEscape(tagID), nil, nil, nil)
}

// CrmFillingAnswers lists CRM filling answers pushed to the CRM for a call
// (GET /calls/{id}/crm-filling-answers).
func (c *Client) CrmFillingAnswers(ctx context.Context, id string, f PageFilter) iter.Seq2[CrmFillingAnswer, error] {
	return paginate[CrmFillingAnswer](ctx, c, "/calls/"+url.PathEscape(id)+"/crm-filling-answers", f.query, f.Limit)
}

// --- call upload ---

// CRMRef points at an object in an external CRM by (crm, crmId). Used to attach
// an account or deal when uploading a call.
type CRMRef struct {
	CRM   string `json:"crm"`
	CRMID string `json:"crmId"`
}

// UploadCallParticipant is one participant on an uploaded call. Type is "user"
// or "contact".
type UploadCallParticipant struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type"`
}

// UploadCallInput is the payload for uploading a call (OpenAPI
// UploadCallRequest). downloadMediaUrl, date and participants are required.
type UploadCallInput struct {
	DownloadMediaURL string                  `json:"downloadMediaUrl"`
	Name             string                  `json:"name,omitempty"`
	Date             string                  `json:"date"`
	Direction        string                  `json:"direction,omitempty"` // inbound | outbound
	Duration         float64                 `json:"duration,omitempty"`
	Participants     []UploadCallParticipant `json:"participants"`
	Tags             []string                `json:"tags,omitempty"`
	Account          *CRMRef                 `json:"account,omitempty"`
	Deal             *CRMRef                 `json:"deal,omitempty"`
}

// UploadCallResponse is the 202 reply to a call upload (OpenAPI
// UploadCallResponse): the new call's id and its processing status.
type UploadCallResponse struct {
	CallID string `json:"callId"`
	Status string `json:"status"`
}

// UploadCall uploads a call via a recording URL (POST /calls -> 202). Modjo
// downloads and processes the recording asynchronously.
func (c *Client) UploadCall(ctx context.Context, in UploadCallInput) (UploadCallResponse, error) {
	body, _ := json.Marshal(in)
	var out UploadCallResponse
	err := c.doJSON(ctx, http.MethodPost, "/calls", nil, bytes.NewReader(body), &out)
	return out, err
}

// --- deal summary ---

// GetDealSummary fetches a deal's AI-generated summary (GET /deals/{id}/summary).
func (c *Client) GetDealSummary(ctx context.Context, id string) (DealSummaryContent, error) {
	var out DealSummaryContent
	err := c.doJSON(ctx, http.MethodGet, "/deals/"+url.PathEscape(id)+"/summary", nil, nil, &out)
	return out, err
}

// --- CRM filling templates ---

// CrmFillingTemplates lists CRM filling templates (GET /crm-filling-templates).
func (c *Client) CrmFillingTemplates(ctx context.Context, f CrmFillingTemplateFilter) iter.Seq2[CrmFillingTemplate, error] {
	return paginate[CrmFillingTemplate](ctx, c, "/crm-filling-templates", f.query, f.Limit)
}

// GetCrmFillingTemplate fetches one CRM filling template by uuid
// (GET /crm-filling-templates/{uuid}).
func (c *Client) GetCrmFillingTemplate(ctx context.Context, uuid string) (CrmFillingTemplate, error) {
	var out CrmFillingTemplate
	err := c.doJSON(ctx, http.MethodGet, "/crm-filling-templates/"+url.PathEscape(uuid), nil, nil, &out)
	return out, err
}

// CrmFillingTemplateFields lists the fields of a CRM filling template
// (GET /crm-filling-templates/{uuid}/fields).
func (c *Client) CrmFillingTemplateFields(ctx context.Context, uuid string, f PageFilter) iter.Seq2[CrmFillingField, error] {
	return paginate[CrmFillingField](ctx, c, "/crm-filling-templates/"+url.PathEscape(uuid)+"/fields", f.query, f.Limit)
}

// --- teams writes + members ---

// CreateTeamInput is the payload for creating a team (OpenAPI CreateTeamRequest).
type CreateTeamInput struct {
	Name string `json:"name"`
}

// CreateTeam creates a team (POST /teams -> 201 Team).
func (c *Client) CreateTeam(ctx context.Context, in CreateTeamInput) (Team, error) {
	body, _ := json.Marshal(in)
	var out Team
	err := c.doJSON(ctx, http.MethodPost, "/teams", nil, bytes.NewReader(body), &out)
	return out, err
}

// UpdateTeamInput is the payload for renaming a team (OpenAPI UpdateTeamRequest).
type UpdateTeamInput struct {
	Name string `json:"name"`
}

// UpdateTeam renames a team (PATCH /teams/{id} -> 200 Team).
func (c *Client) UpdateTeam(ctx context.Context, id string, in UpdateTeamInput) (Team, error) {
	body, _ := json.Marshal(in)
	var out Team
	err := c.doJSON(ctx, http.MethodPatch, "/teams/"+url.PathEscape(id), nil, bytes.NewReader(body), &out)
	return out, err
}

// DeleteTeam soft-deletes a team by id (DELETE /teams/{id} -> 204).
func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/teams/"+url.PathEscape(id), nil, nil, nil)
}

// TeamMembers lists the members of a team (GET /teams/{id}/members).
func (c *Client) TeamMembers(ctx context.Context, id string, f PageFilter) iter.Seq2[TeamMember, error] {
	return paginate[TeamMember](ctx, c, "/teams/"+url.PathEscape(id)+"/members", f.query, f.Limit)
}

// --- user update + team membership ---

// UpdateUserInput is the payload for partially updating a user (OpenAPI
// UpdateUserRequest). Every field is a pointer so the caller can distinguish
// "leave unchanged" (nil, omitted from the body) from "set to this value"
// (non-nil, sent even when the value is empty — e.g. clearing a phone number).
type UpdateUserInput struct {
	Email         *string `json:"email,omitempty"`
	FirstName     *string `json:"firstName,omitempty"`
	LastName      *string `json:"lastName,omitempty"`
	PhoneNumber   *string `json:"phoneNumber,omitempty"`
	JobTitle      *string `json:"jobTitle,omitempty"`
	JobDepartment *string `json:"jobDepartment,omitempty"`
	Role          *string `json:"role,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
}

// UpdateUser partially updates a user (PATCH /users/{id} -> 200 User).
func (c *Client) UpdateUser(ctx context.Context, id string, in UpdateUserInput) (User, error) {
	body, _ := json.Marshal(in)
	var out User
	err := c.doJSON(ctx, http.MethodPatch, "/users/"+url.PathEscape(id), nil, bytes.NewReader(body), &out)
	return out, err
}

// AddUserTeam adds a user to a team (POST /users/{id}/teams -> 201 UserTeam).
// Idempotent: re-adding returns the existing membership.
func (c *Client) AddUserTeam(ctx context.Context, id string, teamID int) (UserTeam, error) {
	body, _ := json.Marshal(addTeamToUserInput{TeamID: teamID})
	var out UserTeam
	err := c.doJSON(ctx, http.MethodPost, "/users/"+url.PathEscape(id)+"/teams", nil, bytes.NewReader(body), &out)
	return out, err
}

// RemoveUserTeam removes a user from a team (DELETE /users/{id}/teams/{teamId}
// -> 204).
func (c *Client) RemoveUserTeam(ctx context.Context, id, teamID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/users/"+url.PathEscape(id)+"/teams/"+url.PathEscape(teamID), nil, nil, nil)
}

// --- webhook update ---

// UpdateWebhookInput is the payload for partially updating a webhook (OpenAPI
// UpdateWebhookRequest). Name and URL are pointers so the caller can send an
// explicit empty value ("clear this field") distinctly from "leave unchanged"
// (nil). Events omits when empty/nil ("leave unchanged"); a non-empty slice
// replaces the subscription set. Clearing all events is not expressible (the
// API treats an absent events field as "no change", not "clear").
type UpdateWebhookInput struct {
	Name   *string  `json:"name,omitempty"`
	URL    *string  `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
}

// UpdateWebhook partially updates a webhook (PATCH /webhooks/{uuid} -> 200).
func (c *Client) UpdateWebhook(ctx context.Context, uuid string, in UpdateWebhookInput) (Webhook, error) {
	body, _ := json.Marshal(in)
	var out Webhook
	err := c.doJSON(ctx, http.MethodPatch, "/webhooks/"+url.PathEscape(uuid), nil, bytes.NewReader(body), &out)
	return out, err
}

// Ptr returns a pointer to v. It's a convenience for building partial-update
// inputs (UpdateUserInput, UpdateWebhookInput) where a nil field means "leave
// unchanged" and a non-nil field — even pointing at an empty value — means
// "set to this".
func Ptr[T any](v T) *T { return &v }

// addTagToCallInput / addTeamToUserInput are the tiny single-field bodies for
// the membership-style POSTs (OpenAPI AddTagToCallRequest / AddTeamToUserRequest).
type addTagToCallInput struct {
	TagID int `json:"tagId"`
}

type addTeamToUserInput struct {
	TeamID int `json:"teamId"`
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
