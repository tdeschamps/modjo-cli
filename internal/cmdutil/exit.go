package cmdutil

import (
	"context"
	"errors"
	"net/http"

	"github.com/tdeschamps/modjo-cli/internal/api"
)

// ErrNotAuthenticated is returned when no credential can be resolved. It maps to
// exit code 3 even when wrapped by the HTTP transport.
var ErrNotAuthenticated = errors.New("not authenticated: run `modjo auth login` (or set MODJO_API_KEY)")

// Process exit codes (product spec §9).
const (
	ExitOK         = 0
	ExitError      = 1
	ExitUsage      = 2
	ExitAuth       = 3
	ExitForbidden  = 4
	ExitNotFound   = 5
	ExitRateLimit  = 6
	ExitValidation = 7
	ExitUpstream   = 8
	ExitTimeout    = 124
)

// UsageError signals a bad invocation (maps to exit code 2). Wrap flag/arg
// problems in this so the root can render usage and exit appropriately.
type UsageError struct{ err error }

// NewUsageError wraps err as a usage error.
func NewUsageError(err error) *UsageError { return &UsageError{err: err} }

func (e *UsageError) Error() string { return e.err.Error() }
func (e *UsageError) Unwrap() error { return e.err }

// SilentError suppresses the default error print (the command already
// reported the problem) while still controlling the exit code.
type SilentError struct {
	Code int
	err  error
}

// NewSilentError wraps err so it exits with code but prints nothing extra.
func NewSilentError(code int, err error) *SilentError { return &SilentError{Code: code, err: err} }

func (e *SilentError) Error() string { return e.err.Error() }
func (e *SilentError) Unwrap() error { return e.err }

// ExitCodeForError maps an error to the appropriate process exit code.
func ExitCodeForError(err error) int {
	if err == nil {
		return ExitOK
	}

	var silent *SilentError
	if errors.As(err, &silent) {
		return silent.Code
	}

	var usage *UsageError
	if errors.As(err, &usage) {
		return ExitUsage
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ExitTimeout
	}

	if errors.Is(err, ErrNotAuthenticated) {
		return ExitAuth
	}

	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return ExitAuth
		case http.StatusForbidden:
			return ExitForbidden
		case http.StatusNotFound:
			return ExitNotFound
		case http.StatusUnprocessableEntity:
			return ExitValidation
		case http.StatusTooManyRequests:
			return ExitRateLimit
		}
		if apiErr.StatusCode >= 500 && apiErr.StatusCode <= 599 {
			return ExitUpstream
		}
		return ExitError
	}

	return ExitError
}
