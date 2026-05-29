package api

import (
	"testing"
)

func TestErrorMessage(t *testing.T) {
	e := &Error{StatusCode: 404, Message: "not found"}
	if e.Error() != "API error 404: not found" {
		t.Errorf("with message = %q", e.Error())
	}
	bare := &Error{StatusCode: 500}
	if bare.Error() != "API error 500" {
		t.Errorf("without message = %q", bare.Error())
	}
}

func TestNewAPIErrorParsesVariants(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"message":"m"}`, "m"},
		{`{"error":"e"}`, "e"},
		{`{"detail":"d"}`, "d"},
		{`not json`, ""},
		{`{}`, ""},
	}
	for _, tc := range cases {
		got := newAPIError(400, "req1", []byte(tc.body))
		if got.Message != tc.want {
			t.Errorf("body %q → message %q want %q", tc.body, got.Message, tc.want)
		}
		if got.RequestID != "req1" || got.StatusCode != 400 {
			t.Errorf("metadata lost: %+v", got)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"open":     "Open",
		"won":      "Closed won",
		"lost":     "Closed lost",
		"closed":   "Closed",
		"deleted":  "Deleted",
		"Whatever": "Whatever",
	}
	for in, want := range cases {
		if got := NormalizeStatus(in); got != want {
			t.Errorf("NormalizeStatus(%q) = %q want %q", in, got, want)
		}
	}
}
