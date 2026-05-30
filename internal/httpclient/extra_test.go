package httpclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("") != 0 {
		t.Error("empty → 0")
	}
	if parseRetryAfter("5") != 5*time.Second {
		t.Error("seconds form")
	}
	if parseRetryAfter("garbage") != 0 {
		t.Error("garbage → 0")
	}
	future := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
	if parseRetryAfter(future) <= 0 {
		t.Error("http-date form should yield a positive duration")
	}
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if parseRetryAfter(past) != 0 {
		t.Error("past http-date → 0")
	}
}

func TestRetryHonorsRetryAfter(t *testing.T) {
	var n int
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n++
		if n == 1 {
			resp := mkResp(429, "slow")
			resp.Header.Set("Retry-After", "0")
			return resp, nil
		}
		return mkResp(200, "ok"), nil
	})
	var slept time.Duration
	rt := &retryRoundTripper{base: base, maxRetries: 2, backoff: defaultBackoff(), sleep: func(d time.Duration) { slept += d }}
	req, _ := http.NewRequest("GET", "https://x", strings.NewReader("body"))
	resp, err := rt.RoundTrip(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestRetryNetworkError(t *testing.T) {
	var n int
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n++
		if n < 2 {
			return nil, errors.New("connection refused")
		}
		return mkResp(200, "ok"), nil
	})
	rt := &retryRoundTripper{base: base, maxRetries: 3, backoff: backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }}, sleep: func(time.Duration) {}}
	req, _ := http.NewRequest("GET", "https://x", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("should recover after network error: %v", err)
	}
	if n != 2 {
		t.Errorf("attempts = %d", n)
	}
}

func TestRetryNetworkErrorExhausted(t *testing.T) {
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("down")
	})
	rt := &retryRoundTripper{base: base, maxRetries: 1, backoff: backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }}, sleep: func(time.Duration) {}}
	req, _ := http.NewRequest("GET", "https://x", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Error("expected error after exhausting retries")
	}
}

func TestRetrySkipsNonIdempotentMethods(t *testing.T) {
	for _, method := range []string{"POST", "PATCH", "DELETE", "PUT"} {
		var calls int
		base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return mkResp(503, "down"), nil
		})
		rt := &retryRoundTripper{base: base, maxRetries: 3, backoff: backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }}, sleep: func(time.Duration) {}}
		req, _ := http.NewRequest(method, "https://x", strings.NewReader("body"))
		if _, err := rt.RoundTrip(req); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Errorf("%s should not be retried: got %d attempts", method, calls)
		}
	}
}

func TestRetryGetIsStillRetried(t *testing.T) {
	var calls int
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return mkResp(503, "down"), nil
	})
	rt := &retryRoundTripper{base: base, maxRetries: 2, backoff: backoff{base: time.Nanosecond, max: time.Nanosecond, jitter: func() float64 { return 0 }}, sleep: func(time.Duration) {}}
	req, _ := http.NewRequest("GET", "https://x", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("GET should retry: got %d attempts, want 3", calls)
	}
}

func TestRetryStopsOnCancelledContext(t *testing.T) {
	var calls int
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return mkResp(503, "down"), nil
	})
	var slept int
	rt := &retryRoundTripper{base: base, maxRetries: 5, backoff: defaultBackoff(), sleep: func(time.Duration) { slept++ }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://x", nil)
	_, err := rt.RoundTrip(req)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled context should surface the context error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("cancelled context should stop retries: got %d attempts", calls)
	}
	if slept != 0 {
		t.Errorf("should not sleep when context is done: slept %d times", slept)
	}
}

func TestRetryNetworkErrorStopsOnCancelledContext(t *testing.T) {
	var calls int
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("boom")
	})
	rt := &retryRoundTripper{base: base, maxRetries: 5, backoff: defaultBackoff(), sleep: func(time.Duration) {}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://x", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Error("expected error")
	}
	if calls != 1 {
		t.Errorf("cancelled context should stop network-error retries: got %d", calls)
	}
}

func TestLoggingErrorPath(t *testing.T) {
	var buf strings.Builder
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, errors.New("boom") })
	rt := &loggingRoundTripper{base: base, out: &buf, redact: true}
	req, _ := http.NewRequest("GET", "https://x", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(buf.String(), "error") {
		t.Errorf("log should mention error: %s", buf.String())
	}
}

func TestLoggingUnsafeShowsAuth(t *testing.T) {
	var buf strings.Builder
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) { return mkResp(200, "ok"), nil })
	rt := &loggingRoundTripper{base: base, out: &buf, redact: false}
	req, _ := http.NewRequest("GET", "https://x", nil)
	req.Header.Set("Authorization", "Bearer visible")
	_, _ = rt.RoundTrip(req)
	if !strings.Contains(buf.String(), "visible") {
		t.Errorf("unsafe logging should show token: %s", buf.String())
	}
}

func TestNewAssemblesChainAndInsecure(t *testing.T) {
	c := New(Options{
		Token:       func() (string, error) { return "t", nil },
		MaxRetries:  2,
		Debug:       true,
		DebugUnsafe: false,
		DebugOut:    &strings.Builder{},
		Insecure:    true,
	})
	if c.Transport == nil {
		t.Error("transport should be set")
	}
}

func TestNewBaseTransportInsecure(t *testing.T) {
	rt := newBaseTransport(true).(*http.Transport)
	if rt.TLSClientConfig == nil || !rt.TLSClientConfig.InsecureSkipVerify {
		t.Error("insecure should skip TLS verify")
	}
	rt2 := newBaseTransport(false).(*http.Transport)
	if rt2.TLSClientConfig != nil && rt2.TLSClientConfig.InsecureSkipVerify {
		t.Error("secure should verify TLS")
	}
}

func TestDelayJitter(t *testing.T) {
	b := backoff{base: 100 * time.Millisecond, max: time.Second, jitter: func() float64 { return 0.5 }}
	d := b.delay(0)
	if d < 100*time.Millisecond || d > 200*time.Millisecond {
		t.Errorf("jittered delay out of range: %v", d)
	}
	// Cap with jitter.
	capped := b.delay(20)
	if capped > time.Second {
		t.Errorf("capped delay exceeded max: %v", capped)
	}
}
