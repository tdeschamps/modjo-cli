package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mkResp(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	// No jitter for determinism.
	b := backoff{base: 100 * time.Millisecond, max: time.Second, jitter: func() float64 { return 0 }}
	got := []time.Duration{b.delay(0), b.delay(1), b.delay(2), b.delay(10)}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delay(%d)=%v want %v", i, got[i], want[i])
		}
	}
}

func TestAuthRoundTripperInjectsBearer(t *testing.T) {
	var seen string
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Header.Get("Authorization")
		return mkResp(200, "ok"), nil
	})
	rt := &authRoundTripper{base: base, token: func() (string, error) { return "secret", nil }}
	req, _ := http.NewRequest("GET", "https://example/v2/me", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if seen != "Bearer secret" {
		t.Errorf("Authorization header = %q", seen)
	}
}

func TestRetryRetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return mkResp(429, "slow down"), nil
		}
		return mkResp(200, "done"), nil
	})
	rt := &retryRoundTripper{
		base:       base,
		maxRetries: 3,
		backoff:    backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }},
		sleep:      func(time.Duration) {},
	}
	req, _ := http.NewRequest("GET", "https://example/v2/calls", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Errorf("calls = %d want 3", calls)
	}
}

func TestRetryGivesUpAfterMax(t *testing.T) {
	var calls int32
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return mkResp(503, "down"), nil
	})
	rt := &retryRoundTripper{
		base:       base,
		maxRetries: 2,
		backoff:    backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }},
		sleep:      func(time.Duration) {},
	}
	req, _ := http.NewRequest("GET", "https://example/v2/calls", nil)
	resp, _ := rt.RoundTrip(req)
	if resp.StatusCode != 503 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	// initial + 2 retries = 3 attempts
	if calls != 3 {
		t.Errorf("calls = %d want 3", calls)
	}
}

func TestLoggingRedactsAuthHeader(t *testing.T) {
	var buf bytes.Buffer
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) { return mkResp(200, "ok"), nil })
	rt := &loggingRoundTripper{base: base, out: &buf, redact: true}
	req, _ := http.NewRequest("GET", "https://example/v2/me", nil)
	req.Header.Set("Authorization", "Bearer topsecret")
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	if strings.Contains(logged, "topsecret") {
		t.Errorf("secret leaked into debug log: %q", logged)
	}
	if !strings.Contains(logged, "REDACTED") {
		t.Errorf("expected REDACTED marker, got %q", logged)
	}
}
