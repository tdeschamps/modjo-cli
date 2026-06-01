package api

import (
	"encoding/json"
	"strings"
)

// Deal status canonical values (OpenAPI Deal.status enum).
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

// TranscriptBlock is one speaker-labeled, timestamped segment of a call. The
// API nests the speaker in a sub-object; UnmarshalJSON flattens its name onto
// SpeakerName so the renderer stays simple.
type TranscriptBlock struct {
	StartTime   float64 `json:"startTime"`
	EndTime     float64 `json:"endTime"`
	SpeakerName string  `json:"speakerName"`
	Content     string  `json:"content"`
}

// UnmarshalJSON reads the API's transcript block shape ({speaker:{name}}).
func (b *TranscriptBlock) UnmarshalJSON(data []byte) error {
	var raw struct {
		StartTime float64 `json:"startTime"`
		EndTime   float64 `json:"endTime"`
		Content   string  `json:"content"`
		Speaker   struct {
			Name string `json:"name"`
		} `json:"speaker"`
		// Tolerate a pre-flattened speakerName too.
		SpeakerName string `json:"speakerName"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.StartTime, b.EndTime, b.Content = raw.StartTime, raw.EndTime, raw.Content
	if b.SpeakerName = raw.SpeakerName; b.SpeakerName == "" {
		b.SpeakerName = raw.Speaker.Name
	}
	return nil
}

// Call is a recorded conversation (OpenAPI CallExpanded). The title field is
// "name" and the timestamp "date"; json tags map those onto Title/StartTime so
// the renderer needn't care which endpoint produced the value. Related entities
// arrive as either IDs or expanded objects (with ?expand=); both are kept.
type Call struct {
	ID                  json.Number       `json:"id"`
	Title               string            `json:"name"`
	StartTime           string            `json:"date"`
	Duration            float64           `json:"duration"`
	Direction           string            `json:"direction,omitempty"`
	Language            string            `json:"language,omitempty"`
	Status              string            `json:"status,omitempty"`
	PhoneProviderCallID string            `json:"phoneProviderCallId,omitempty"`
	AccountID           json.Number       `json:"accountId,omitempty"`
	DealID              json.Number       `json:"dealId,omitempty"`
	ContactIDs          []json.Number     `json:"contactIds,omitempty"`
	UserIDs             []json.Number     `json:"userIds,omitempty"`
	Account             json.RawMessage   `json:"account,omitempty"`
	Deal                json.RawMessage   `json:"deal,omitempty"`
	Contacts            json.RawMessage   `json:"contacts,omitempty"`
	Users               json.RawMessage   `json:"users,omitempty"`
	CRMActivity         json.RawMessage   `json:"crmActivity,omitempty"`
	CreatedOn           string            `json:"createdOn,omitempty"`
	ModifiedOn          string            `json:"modifiedOn,omitempty"`
	Transcript          []TranscriptBlock `json:"transcript,omitempty"`
}

// Deal is a CRM opportunity (OpenAPI Deal). The primary key is numeric ID;
// crmId is the external CRM identifier.
type Deal struct {
	ID            json.Number `json:"id"`
	Name          string      `json:"name"`
	CRMID         string      `json:"crmId,omitempty"`
	CRMIdentifier string      `json:"crmIdentifier,omitempty"`
	CRMLink       string      `json:"crmLink,omitempty"`
	CRM           string      `json:"crm,omitempty"`
	Status        string      `json:"status,omitempty"`
	Stage         string      `json:"stage,omitempty"`
	CloseDate     string      `json:"closeDate,omitempty"`
	Amount        float64     `json:"amount,omitempty"`
	Currency      string      `json:"currency,omitempty"`
	Probability   float64     `json:"probability,omitempty"`
	LossReason    string      `json:"lossReason,omitempty"`
	Source        string      `json:"source,omitempty"`
	AccountID     json.Number `json:"accountId,omitempty"`
	OwnerID       json.Number `json:"ownerId,omitempty"`
	CreatedOn     string      `json:"createdOn,omitempty"`
	ModifiedOn    string      `json:"modifiedOn,omitempty"`
}

// Account is a company/organization (OpenAPI Account).
type Account struct {
	ID            json.Number `json:"id"`
	Name          string      `json:"name"`
	CRMID         string      `json:"crmId,omitempty"`
	CRMIdentifier string      `json:"crmIdentifier,omitempty"`
	CRMLink       string      `json:"crmLink,omitempty"`
	CRM           string      `json:"crm,omitempty"`
	CreatedOn     string      `json:"createdOn,omitempty"`
	ModifiedOn    string      `json:"modifiedOn,omitempty"`
}

// Contact is a person (OpenAPI Contact).
type Contact struct {
	ID            json.Number `json:"id"`
	Name          string      `json:"name,omitempty"`
	Email         string      `json:"email,omitempty"`
	PhoneNumber   string      `json:"phoneNumber,omitempty"`
	JobTitle      string      `json:"jobTitle,omitempty"`
	CRMPersonID   string      `json:"crmPersonId,omitempty"`
	CRMIdentifier string      `json:"crmIdentifier,omitempty"`
	CRMLink       string      `json:"crmLink,omitempty"`
	CRM           string      `json:"crm,omitempty"`
	AccountID     json.Number `json:"accountId,omitempty"`
	CreatedOn     string      `json:"createdOn,omitempty"`
	ModifiedOn    string      `json:"modifiedOn,omitempty"`
}

// User is a Modjo workspace member (OpenAPI User). The API returns separate
// firstName/lastName and jobDepartment/jobTitle fields; UnmarshalJSON folds
// those into the flat Name/Department/Title the CLI displays, while still
// accepting a pre-composed "name" if the API ever sends one.
// The json tags drive marshaling (so `--json` emits the folded fields);
// UnmarshalJSON below ignores them and reads the API's raw shape on decode.
type User struct {
	ID          json.Number `json:"id"`
	Email       string      `json:"email"`
	Name        string      `json:"name,omitempty"`
	Role        string      `json:"role,omitempty"`
	Department  string      `json:"department,omitempty"`
	Title       string      `json:"title,omitempty"`
	PhoneNumber string      `json:"phoneNumber,omitempty"`
	Timezone    string      `json:"timezone,omitempty"`
	CreatedOn   string      `json:"createdOn,omitempty"`
	ModifiedOn  string      `json:"modifiedOn,omitempty"`
}

// UnmarshalJSON maps the API's user shape onto User.
func (u *User) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID            json.Number `json:"id"`
		Email         string      `json:"email"`
		Name          string      `json:"name"`
		FirstName     string      `json:"firstName"`
		LastName      string      `json:"lastName"`
		Role          string      `json:"role"`
		Department    string      `json:"department"`
		JobDepartment string      `json:"jobDepartment"`
		JobTitle      string      `json:"jobTitle"`
		PhoneNumber   string      `json:"phoneNumber"`
		Timezone      string      `json:"timezone"`
		CreatedOn     string      `json:"createdOn"`
		ModifiedOn    string      `json:"modifiedOn"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	u.ID = raw.ID
	u.Email = raw.Email
	u.Role = raw.Role
	u.PhoneNumber = raw.PhoneNumber
	u.Timezone = raw.Timezone
	u.CreatedOn = raw.CreatedOn
	u.ModifiedOn = raw.ModifiedOn
	u.Title = raw.JobTitle
	if u.Department = raw.Department; u.Department == "" {
		u.Department = raw.JobDepartment
	}
	if u.Name = raw.Name; u.Name == "" {
		u.Name = strings.TrimSpace(raw.FirstName + " " + raw.LastName)
	}
	return nil
}

// Team is a group of users (OpenAPI Team).
type Team struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	CreatedOn  string      `json:"createdOn,omitempty"`
	ModifiedOn string      `json:"modifiedOn,omitempty"`
}

// Tag is a call tag (OpenAPI Tag).
type Tag struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Color      string      `json:"color,omitempty"`
	CreatedOn  string      `json:"createdOn,omitempty"`
	ModifiedOn string      `json:"modifiedOn,omitempty"`
}

// Topic is a conversation topic (OpenAPI Topic).
type Topic struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Slug       string      `json:"slug,omitempty"`
	Color      string      `json:"color,omitempty"`
	SaidBy     string      `json:"saidBy,omitempty"` // user | contact
	CreatedOn  string      `json:"createdOn,omitempty"`
	ModifiedOn string      `json:"modifiedOn,omitempty"`
}

// Webhook is an event subscription (OpenAPI Webhook). Its primary key is a
// UUID, not a numeric id.
type Webhook struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Events     []string `json:"events,omitempty"` // call_summarized | call_recording_deleted | call_transcript_deleted
	CreatedOn  string   `json:"createdOn,omitempty"`
	ModifiedOn string   `json:"modifiedOn,omitempty"`
}

// CallSummary is one generated summary for a call (OpenAPI CallSummary).
type CallSummary struct {
	UUID           string `json:"uuid"`
	TemplateUUID   string `json:"templateUuid,omitempty"`
	TemplateTitle  string `json:"templateTitle,omitempty"`
	TemplateLength string `json:"templateLength,omitempty"` // short | detailed | adjusted
	Answer         string `json:"answer,omitempty"`
	Language       string `json:"language,omitempty"`
	CreatedOn      string `json:"createdOn,omitempty"`
	ModifiedOn     string `json:"modifiedOn,omitempty"`
}

// NextStepItem is one action item extracted from a call (OpenAPI NextStepItem).
type NextStepItem struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Note is a call note (OpenAPI Note).
type Note struct {
	ID            json.Number     `json:"id"`
	Title         string          `json:"title,omitempty"`
	Date          string          `json:"date,omitempty"`
	RawContent    json.RawMessage `json:"rawContent,omitempty"`
	Status        string          `json:"status,omitempty"` // DRAFT | PUBLISHED
	Type          string          `json:"type,omitempty"`   // USER | AI
	PublishedDate string          `json:"publishedDate,omitempty"`
	CreatedOn     string          `json:"createdOn,omitempty"`
	ModifiedOn    string          `json:"modifiedOn,omitempty"`
	CreatedBy     json.RawMessage `json:"createdBy,omitempty"`
	ModifiedBy    json.RawMessage `json:"modifiedBy,omitempty"`
}
