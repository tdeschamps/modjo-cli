package api

import "encoding/json"

// Deal status canonical values (product spec §7.3).
const (
	StatusOpen       = "Open"
	StatusClosedWon  = "Closed won"
	StatusClosedLost = "Closed lost"
	StatusClosed     = "Closed"
	StatusDeleted    = "Deleted"
)

// NormalizeStatus maps the CLI's convenience aliases to the API's canonical
// status strings. Unknown values pass through unchanged.
func NormalizeStatus(s string) string {
	switch s {
	case "open":
		return StatusOpen
	case "won":
		return StatusClosedWon
	case "lost":
		return StatusClosedLost
	case "closed":
		return StatusClosed
	case "deleted":
		return StatusDeleted
	default:
		return s
	}
}

// TranscriptBlock is one speaker-labeled, timestamped segment of a call.
type TranscriptBlock struct {
	StartTime   float64 `json:"startTime"`
	EndTime     float64 `json:"endTime"`
	SpeakerName string  `json:"speakerName"`
	Content     string  `json:"content"`
}

// Call is a recorded conversation.
type Call struct {
	ID         json.Number       `json:"id"`
	Title      string            `json:"title"`
	StartTime  string            `json:"startDate"`
	Duration   float64           `json:"duration"`
	Summary    string            `json:"summary,omitempty"`
	Language   string            `json:"language,omitempty"`
	CRMLink    string            `json:"crmLink,omitempty"`
	Speakers   []Speaker         `json:"speakers,omitempty"`
	Transcript []TranscriptBlock `json:"transcript,omitempty"`
	Relations  json.RawMessage   `json:"relations,omitempty"`
}

// Speaker is a participant in a call.
type Speaker struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Type  string `json:"type,omitempty"` // internal | external
}

// Deal is a CRM opportunity.
type Deal struct {
	CRMID      string  `json:"crmId"`
	Name       string  `json:"name"`
	AccountID  string  `json:"accountCrmId,omitempty"`
	Account    string  `json:"accountName,omitempty"`
	Status     string  `json:"status"`
	Amount     float64 `json:"amount,omitempty"`
	Currency   string  `json:"currency,omitempty"`
	CloseDate  string  `json:"closeDate,omitempty"`
	Source     string  `json:"source,omitempty"`
	LossReason string  `json:"lossReason,omitempty"`
	CRMLink    string  `json:"crmLink,omitempty"`
}

// Account is a company/organization.
type Account struct {
	CRMID   string `json:"crmId"`
	Name    string `json:"name"`
	Domain  string `json:"domain,omitempty"`
	CRMLink string `json:"crmLink,omitempty"`
}

// Contact is a person.
type Contact struct {
	CRMPersonID string `json:"crmPersonId"`
	Name        string `json:"name"`
	Email       string `json:"email,omitempty"`
	Title       string `json:"title,omitempty"`
	AccountID   string `json:"accountCrmId,omitempty"`
}

// Email is a tracked email message.
type Email struct {
	ID        json.Number `json:"id"`
	Subject   string      `json:"subject"`
	From      string      `json:"from,omitempty"`
	To        []string    `json:"to,omitempty"`
	Date      string      `json:"date,omitempty"`
	AccountID string      `json:"accountCrmId,omitempty"`
	DealID    string      `json:"dealCrmId,omitempty"`
	Content   string      `json:"content,omitempty"`
}

// User is a Modjo workspace member.
type User struct {
	ID         json.Number `json:"id"`
	Email      string      `json:"email"`
	Name       string      `json:"name,omitempty"`
	Role       string      `json:"role,omitempty"`
	Department string      `json:"department,omitempty"`
	TeamID     json.Number `json:"teamId,omitempty"`
}

// Team is a group of users.
type Team struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

// Agent is a native or custom Modjo analysis agent.
type Agent struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Origin      string `json:"origin,omitempty"` // modjo | user
}
