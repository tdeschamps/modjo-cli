// Package httpclient builds the *http.Client used for all REST traffic. Its
// Transport is a composable chain of RoundTrippers — auth → retry → logging —
// each independently testable by feeding a fake base RoundTripper.
package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// Options configures the client chain.
type Options struct {
	// Token returns the current bearer token (called per-request so OAuth
	// refresh is transparent). If nil, no Authorization header is added.
	Token func() (string, error)
	// MaxRetries caps automatic retries on 429/5xx. 0 disables retries.
	MaxRetries int
	// Debug enables request/response logging to DebugOut.
	Debug bool
	// DebugUnsafe shows the Authorization header in logs (default redacted).
	DebugUnsafe bool
	// DebugOut is where debug logs go (typically stderr).
	DebugOut io.Writer
	// Insecure skips TLS verification (self-hosted/proxy debugging only).
	Insecure bool
	// Base lets tests inject a fake transport. Defaults to http.DefaultTransport.
	Base http.RoundTripper
}

// New assembles an *http.Client from Options.
func New(opt Options) *http.Client {
	base := opt.Base
	if base == nil {
		base = newBaseTransport(opt.Insecure)
	}

	rt := base

	if opt.MaxRetries > 0 {
		rt = &retryRoundTripper{
			base:       rt,
			maxRetries: opt.MaxRetries,
			backoff:    defaultBackoff(),
			sleep:      time.Sleep,
		}
	}

	if opt.Token != nil {
		rt = &authRoundTripper{base: rt, token: opt.Token}
	}

	if opt.Debug && opt.DebugOut != nil {
		rt = &loggingRoundTripper{base: rt, out: opt.DebugOut, redact: !opt.DebugUnsafe}
	}

	return &http.Client{Transport: rt, Timeout: 0}
}

// authRoundTripper injects the bearer token. It clones the request so we never
// mutate the caller's request (important for retries).
type authRoundTripper struct {
	base  http.RoundTripper
	token func() (string, error)
}

func (a *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := a.token()
	if err != nil {
		return nil, err
	}
	r := req.Clone(req.Context())
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	return a.base.RoundTrip(r)
}

// retryRoundTripper retries idempotent-ish requests on 429 and 5xx with
// exponential backoff + jitter, honoring Retry-After.
type retryRoundTripper struct {
	base       http.RoundTripper
	maxRetries int
	backoff    backoff
	sleep      func(time.Duration)
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body so we can replay it across attempts.
	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		bodyBytes = b
	}

	// Only retry idempotent methods. Replaying a POST/PATCH (user create, an AI
	// ask, an MCP tools/call) after a 5xx the server may have already committed
	// would duplicate the side effect, so those get a single attempt.
	maxRetries := rt.maxRetries
	if !idempotent(req.Method) {
		maxRetries = 0
	}

	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		r := req.Clone(req.Context())
		if bodyBytes != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		resp, err = rt.base.RoundTrip(r)
		if err != nil {
			// Don't retry once retries are exhausted, or once the caller's
			// context is cancelled/expired (the error is terminal anyway).
			if attempt >= maxRetries || req.Context().Err() != nil {
				return nil, err
			}
			rt.sleep(rt.backoff.delay(attempt))
			continue
		}
		if !retryable(resp.StatusCode) || attempt >= maxRetries {
			return resp, nil
		}
		// Don't sleep to retry if the caller's context is already done; surface
		// the context error so a deadline maps to a timeout, not a stale 5xx.
		if cerr := req.Context().Err(); cerr != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil, cerr
		}
		wait := rt.backoff.delay(attempt)
		if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra > 0 {
			wait = ra
		}
		// Drain and close so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		rt.sleep(wait)
	}
}

func retryable(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

// idempotent reports whether a request method is safe to retry automatically.
func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// loggingRoundTripper writes a one-line trace per request/response.
type loggingRoundTripper struct {
	base   http.RoundTripper
	out    io.Writer
	redact bool
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	auth := req.Header.Get("Authorization")
	shown := auth
	if l.redact && auth != "" {
		shown = "Bearer REDACTED"
	}
	start := time.Now()
	fmt.Fprintf(l.out, "> %s %s  Authorization: %s\n", req.Method, req.URL, shown)
	resp, err := l.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(l.out, "< error after %s: %v\n", time.Since(start).Round(time.Millisecond), err)
		return nil, err
	}
	reqID := resp.Header.Get("X-Request-Id")
	fmt.Fprintf(l.out, "< %d %s in %s  request-id=%s\n", resp.StatusCode, req.URL.Path, time.Since(start).Round(time.Millisecond), reqID)
	return resp, nil
}

// backoff computes exponential delays with optional jitter, capped at max.
type backoff struct {
	base   time.Duration
	max    time.Duration
	jitter func() float64 // returns [0,1); added as a fraction of the delay
}

func defaultBackoff() backoff {
	// Retry jitter is not security-sensitive; a fast PRNG is appropriate.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return backoff{base: 500 * time.Millisecond, max: 20 * time.Second, jitter: r.Float64}
}

func (b backoff) delay(attempt int) time.Duration {
	d := float64(b.base) * math.Pow(2, float64(attempt))
	if d > float64(b.max) {
		d = float64(b.max)
	}
	if b.jitter != nil {
		d += d * 0.25 * b.jitter()
		if d > float64(b.max) {
			d = float64(b.max)
		}
	}
	return time.Duration(d)
}
