package api

import (
	"net/url"
	"strconv"
	"strings"
)

// Filters translate CLI flags into API query parameters. Each carries an
// optional Limit; a zero Limit means "no cap" (the iterator pages until the API
// is exhausted). The query() methods take the pagination cursor.

// CallFilter filters the calls list.
type CallFilter struct {
	Account   string
	Deal      string
	Contact   string
	User      string
	Since     string // YYYY-MM-DD
	Until     string // YYYY-MM-DD
	Relations []string
	Limit     int
}

func (f CallFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "accountCrmId", f.Account)
	setNonEmpty(q, "dealCrmId", f.Deal)
	setNonEmpty(q, "contactCrmId", f.Contact)
	setNonEmpty(q, "userId", f.User)
	setNonEmpty(q, "startDate", f.Since)
	setNonEmpty(q, "endDate", f.Until)
	if len(f.Relations) > 0 {
		q.Set("relations", strings.Join(f.Relations, ","))
	}
	setPaging(q, f.Limit, cursor)
	return q
}

// DealFilter filters the deals list.
type DealFilter struct {
	Status      []string
	Account     string
	CloseBefore string
	CloseAfter  string
	AmountMin   float64
	AmountMax   float64
	Source      []string
	LossReason  string
	Limit       int
}

func (f DealFilter) query(cursor string) url.Values {
	q := url.Values{}
	for _, s := range f.Status {
		q.Add("status", NormalizeStatus(s))
	}
	setNonEmpty(q, "accountCrmId", f.Account)
	setNonEmpty(q, "closeDateBefore", f.CloseBefore)
	setNonEmpty(q, "closeDateAfter", f.CloseAfter)
	if f.AmountMin > 0 {
		q.Set("amountMin", strconv.FormatFloat(f.AmountMin, 'f', -1, 64))
	}
	if f.AmountMax > 0 {
		q.Set("amountMax", strconv.FormatFloat(f.AmountMax, 'f', -1, 64))
	}
	for _, s := range f.Source {
		q.Add("source", s)
	}
	setNonEmpty(q, "lossReason", f.LossReason)
	setPaging(q, f.Limit, cursor)
	return q
}

// AccountFilter filters the accounts list.
type AccountFilter struct {
	Name  string
	Limit int
}

func (f AccountFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setPaging(q, f.Limit, cursor)
	return q
}

// ContactFilter filters the contacts list.
type ContactFilter struct {
	Name    string
	Account string
	Limit   int
}

func (f ContactFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setNonEmpty(q, "accountCrmId", f.Account)
	setPaging(q, f.Limit, cursor)
	return q
}

// EmailFilter filters the emails list.
type EmailFilter struct {
	Account string
	Deal    string
	Since   string
	Until   string
	Limit   int
}

func (f EmailFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "accountCrmId", f.Account)
	setNonEmpty(q, "dealCrmId", f.Deal)
	setNonEmpty(q, "startDate", f.Since)
	setNonEmpty(q, "endDate", f.Until)
	setPaging(q, f.Limit, cursor)
	return q
}

// UserFilter filters the users list.
type UserFilter struct {
	Name       string
	Email      string
	Role       string
	Department string
	Limit      int
}

func (f UserFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "name", f.Name)
	setNonEmpty(q, "email", f.Email)
	setNonEmpty(q, "role", f.Role)
	setNonEmpty(q, "department", f.Department)
	setPaging(q, f.Limit, cursor)
	return q
}

// AgentFilter filters the agents list.
type AgentFilter struct {
	Search string
	Origin string // modjo | user
	Limit  int
}

func (f AgentFilter) query(cursor string) url.Values {
	q := url.Values{}
	setNonEmpty(q, "search", f.Search)
	setNonEmpty(q, "origin", f.Origin)
	setPaging(q, f.Limit, cursor)
	return q
}

func setNonEmpty(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

// pageSize is the API's documented maximum page size.
const pageSize = 50

func setPaging(q url.Values, limit int, cursor string) {
	per := pageSize
	if limit > 0 && limit < pageSize {
		per = limit
	}
	q.Set("limit", strconv.Itoa(per))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
}
