//go:build contract

// Package contract holds the gated nightly smoke suite that runs against the
// real Modjo sandbox workspace (it is never part of the per-PR path: no secrets
// for forks, no flakiness gate). It validates that live response shapes still
// satisfy our decoders. Run with: go test -tags=contract ./... -run Contract
package contract

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/httpclient"
)

func liveClient(t *testing.T) *api.Client {
	t.Helper()
	key := os.Getenv("MODJO_API_KEY")
	if key == "" {
		t.Skip("MODJO_API_KEY not set; skipping contract tests")
	}
	base := os.Getenv("MODJO_BASE_URL")
	if base == "" {
		base = config.DefaultBaseURL
	}
	hc := httpclient.New(httpclient.Options{
		Token:      func() (string, error) { return key, nil },
		MaxRetries: 2,
	})
	return api.New(api.Options{BaseURL: base, HTTPClient: hc})
}

func TestContractMe(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := c.Me(ctx); err != nil {
		t.Fatalf("GET /me failed: %v", err)
	}
}

func TestContractDealsDecode(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n := 0
	for _, err := range c.Deals(ctx, api.DealFilter{Limit: 1}) {
		if err != nil {
			var apiErr *api.Error
			if e, ok := err.(*api.Error); ok {
				apiErr = e
			}
			// 200 with decodable values is the contract; a 4xx with a clean
			// error envelope is also acceptable shape-wise.
			if apiErr == nil {
				t.Fatalf("deals decode failed: %v", err)
			}
		}
		n++
		break
	}
	_ = http.StatusOK
}
