package updatecheck

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.0", "v1.0.0", true},
		{"1.0.1", "1.0.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.2.0", false},
		{"v1.2.3-rc1", "v1.2.2", true}, // prerelease suffix stripped
		{"garbage", "v1.0.0", false},   // unparseable → never nag
		{"v1.0", "v1.0.0", false},      // not 3 parts
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q,%q)=%v want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNotice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")

	// No cache → no notice.
	if n := Notice("v1.0.0", path); n != "" {
		t.Errorf("empty cache should give no notice, got %q", n)
	}

	// Seed a newer version.
	_ = writeState(path, state{CheckedAt: time.Now(), Latest: "v1.5.0"})
	if n := Notice("v1.0.0", path); n == "" {
		t.Error("expected a notice for a newer cached version")
	}
	// Same/older current → no notice.
	if n := Notice("v1.5.0", path); n != "" {
		t.Errorf("up-to-date should give no notice, got %q", n)
	}
	// Dev builds never nag.
	if n := Notice("dev", path); n != "" {
		t.Errorf("dev build should not nag, got %q", n)
	}
}

func TestRefreshFetchesWhenStaleOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	now := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)

	calls := 0
	fetch := func(context.Context) (string, error) { calls++; return "v1.2.0", nil }

	// Empty cache → fetch and store.
	Refresh(context.Background(), "v1.0.0", path, fetch, now)
	if calls != 1 || readState(path).Latest != "v1.2.0" {
		t.Fatalf("first refresh: calls=%d state=%+v", calls, readState(path))
	}

	// Fresh cache (within interval) → no fetch.
	Refresh(context.Background(), "v1.0.0", path, fetch, now.Add(time.Hour))
	if calls != 1 {
		t.Errorf("fresh cache should not refetch, calls=%d", calls)
	}

	// Stale cache → fetch again.
	Refresh(context.Background(), "v1.0.0", path, fetch, now.Add(25*time.Hour))
	if calls != 2 {
		t.Errorf("stale cache should refetch, calls=%d", calls)
	}
}

func TestRefreshIsBestEffort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	now := time.Now()

	// Dev build → never fetches.
	called := false
	Refresh(context.Background(), "dev", path, func(context.Context) (string, error) { called = true; return "v9", nil }, now)
	if called {
		t.Error("dev build should not fetch")
	}

	// Fetch error → no state written, no panic.
	Refresh(context.Background(), "v1.0.0", path, func(context.Context) (string, error) {
		return "", errors.New("network down")
	}, now)
	if readState(path).Latest != "" {
		t.Error("failed fetch should not write state")
	}

	// nil fetch → no-op.
	Refresh(context.Background(), "v1.0.0", path, nil, now)
}

func TestStatePathAndSuppressed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("APPDATA", "")
	if p := StatePath(); p == "" || filepath.Base(p) != "update-check.json" {
		t.Errorf("StatePath = %q", p)
	}

	t.Setenv("MODJO_NO_UPDATE_NOTIFIER", "")
	if Suppressed() {
		t.Error("should not be suppressed by default")
	}
	t.Setenv("MODJO_NO_UPDATE_NOTIFIER", "1")
	if !Suppressed() {
		t.Error("MODJO_NO_UPDATE_NOTIFIER should suppress")
	}
}
