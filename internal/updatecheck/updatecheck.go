// Package updatecheck implements a non-blocking "new version available" notice.
// It prints from a small on-disk cache (so it never adds latency) and refreshes
// that cache in the background, mirroring the gh CLI's approach. The check is
// suppressible via MODJO_NO_UPDATE_NOTIFIER and never blocks or fails a command.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// checkInterval is how long a cached result is considered fresh.
const checkInterval = 24 * time.Hour

// state is the cached result of the last check.
type state struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"`
}

// FetchFunc returns the latest released version tag (e.g. "v1.2.3").
type FetchFunc func(ctx context.Context) (string, error)

// readState loads the cache; a missing/corrupt file yields a zero state.
func readState(path string) state {
	data, err := os.ReadFile(path)
	if err != nil {
		return state{}
	}
	var s state
	_ = json.Unmarshal(data, &s)
	return s
}

func writeState(path string, s state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Notice returns a one-line upgrade message when the cached latest version is
// newer than current, or "" otherwise. It is pure and instant — safe to call on
// every command.
func Notice(current, statePath string) string {
	if isDev(current) {
		return ""
	}
	s := readState(statePath)
	if s.Latest == "" || !isNewer(s.Latest, current) {
		return ""
	}
	return fmt.Sprintf("A new release of modjo is available: %s → %s\nhttps://github.com/tdeschamps/modjo-cli/releases/latest",
		normalize(current), normalize(s.Latest))
}

// Refresh fetches the latest version and updates the cache when the cached
// result is stale. It is meant to run in a background goroutine; it returns
// quickly when the cache is fresh and swallows all errors (best-effort).
func Refresh(ctx context.Context, current, statePath string, fetch FetchFunc, now time.Time) {
	if isDev(current) || fetch == nil {
		return
	}
	s := readState(statePath)
	if !s.CheckedAt.IsZero() && now.Sub(s.CheckedAt) < checkInterval {
		return
	}
	latest, err := fetch(ctx)
	if err != nil || latest == "" {
		return
	}
	_ = writeState(statePath, state{CheckedAt: now, Latest: latest})
}

func isDev(v string) bool {
	switch strings.TrimSpace(v) {
	case "", "dev", "none", "unknown":
		return true
	}
	return false
}

func normalize(v string) string { return "v" + strings.TrimPrefix(strings.TrimSpace(v), "v") }

// isNewer reports whether latest is a strictly greater semver than current.
// Non-semver inputs compare false (we never nag on something we can't parse).
func isNewer(latest, current string) bool {
	lp, lok := parseSemver(latest)
	cp, cok := parseSemver(current)
	if !lok || !cok {
		return false
	}
	for i := 0; i < 3; i++ {
		if lp[i] != cp[i] {
			return lp[i] > cp[i]
		}
	}
	return false
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// Drop any pre-release/build suffix.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
