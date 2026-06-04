package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Error is a structured API error carrying the HTTP status, the upstream
// message, and the request ID (for support). The command layer maps StatusCode
// to a process exit code.
type Error struct {
	StatusCode int
	Message    string
	RequestID  string
	Body       string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

// newAPIError parses an error response body into an Error.
func newAPIError(status int, requestID string, body []byte) *Error {
	e := &Error{StatusCode: status, RequestID: requestID, Body: string(body)}
	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		switch {
		case parsed.Message != "":
			e.Message = parsed.Message
		case parsed.Error != "":
			e.Message = parsed.Error
		case parsed.Detail != "":
			e.Message = parsed.Detail
		}
	}
	return e
}

// asError is a thin wrapper around errors.As used by tests.
func asError(err error, target **Error) bool {
	return errors.As(err, target)
}
