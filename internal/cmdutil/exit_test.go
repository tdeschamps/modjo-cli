package cmdutil

import (
	"context"
	"errors"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/api"
)

func TestExitCodeForError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"generic", errors.New("boom"), 1},
		{"usage", &UsageError{err: errors.New("bad flag")}, 2},
		{"auth 401", &api.Error{StatusCode: 401}, 3},
		{"authz 403", &api.Error{StatusCode: 403}, 4},
		{"notfound 404", &api.Error{StatusCode: 404}, 5},
		{"ratelimit 429", &api.Error{StatusCode: 429}, 6},
		{"validation 422", &api.Error{StatusCode: 422}, 7},
		{"server 500", &api.Error{StatusCode: 500}, 8},
		{"server 503", &api.Error{StatusCode: 503}, 8},
		{"timeout", context.DeadlineExceeded, 124},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeForError(tc.err); got != tc.want {
				t.Errorf("ExitCodeForError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
