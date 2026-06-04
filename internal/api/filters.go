package api

import (
	"net/url"
	"strconv"
)

// Filters translate CLI flags into API query parameters. Each carries an
// optional Limit; a zero Limit means "no cap" (the iterator pages until the API
// is exhausted). The query() methods take the 1-indexed page number. Query
// params follow the OpenAPI spec exactly (snake_case, numeric ids).

// CallFilter filters the calls list (GET /calls).
type CallFilter struct {
	Account string   // account_id (numeric)
	Deal    string   // deal_id (numeric)
	User    string   // user_id (numeric)
	Since   string   // from (ISO date-time)
	Until   string   // to (ISO date-time)
	Expand  []string // expand: contacts,deal,account,users
	Limit   int
}

func (f CallFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "account_id", f.Account)
	setNonEmpty(q, "deal_id", f.Deal)
	setNonEmpty(q, "user_id", f.User)
	setNonEmpty(q, "from", f.Since)
	setNonEmpty(q, "to", f.Until)
	for _, e := range f.Expand {
		q.Add("expand", e)
	}
	setPaging(q, f.Limit, page)
	return q
}

// DealFilter filters the deals list (GET /deals).
type DealFilter struct {
	Name    string // name
	Account string // account_id (numeric)
	Status  string // status (single enum value)
	Limit   int
}

func (f DealFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setNonEmpty(q, "account_id", f.Account)
	if f.Status != "" {
		q.Set("status", NormalizeStatus(f.Status))
	}
	setPaging(q, f.Limit, page)
	return q
}

// AccountFilter filters the accounts list (GET /accounts).
type AccountFilter struct {
	Name  string
	Limit int
}

func (f AccountFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setPaging(q, f.Limit, page)
	return q
}

// ContactFilter filters the contacts list (GET /contacts).
type ContactFilter struct {
	Name  string
	Limit int
}

func (f ContactFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setPaging(q, f.Limit, page)
	return q
}

// UserFilter filters the users list (GET /users). The API filters users by
// email only (exact match).
type UserFilter struct {
	Email string
	Limit int
}

func (f UserFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "email", f.Email)
	setPaging(q, f.Limit, page)
	return q
}

// TeamFilter filters the teams list (GET /teams).
type TeamFilter struct {
	Name  string
	Limit int
}

func (f TeamFilter) query(page int) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setPaging(q, f.Limit, page)
	return q
}

// TagFilter filters the tags list (GET /tags). Only paging is supported.
type TagFilter struct {
	Limit int
}

func (f TagFilter) query(page int) url.Values {
	q := url.Values{}
	setPaging(q, f.Limit, page)
	return q
}

// TopicFilter filters the topics list (GET /topics). Only paging is supported.
type TopicFilter struct {
	Limit int
}

func (f TopicFilter) query(page int) url.Values {
	q := url.Values{}
	setPaging(q, f.Limit, page)
	return q
}

// WebhookFilter filters the webhooks list (GET /webhooks). Only paging is
// supported.
type WebhookFilter struct {
	Limit int
}

func (f WebhookFilter) query(page int) url.Values {
	q := url.Values{}
	setPaging(q, f.Limit, page)
	return q
}

func setNonEmpty(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

// pageSize is the API's maximum page size; we request full pages and let the
// iterator stop once the caller's limit (or the dataset total) is reached.
const pageSize = 50

// setPaging sets the page-based query params the API expects: a 1-indexed
// `page` and a `size`. When a small limit is requested we shrink the page so a
// single request suffices; otherwise we pull full pages.
func setPaging(q url.Values, limit, page int) {
	size := pageSize
	if limit > 0 && limit < pageSize {
		size = limit
	}
	q.Set("size", strconv.Itoa(size))
	q.Set("page", strconv.Itoa(page))
}
