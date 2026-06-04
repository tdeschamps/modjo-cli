package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const releasesAPI = "https://api.github.com/repos/tdeschamps/modjo-cli/releases/latest"

// StatePath returns the cache file location (next to the config dir).
func StatePath() string {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			base = xdg
		} else if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		}
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "modjo", "update-check.json")
}

// GitHubLatest fetches the latest release tag from GitHub. It is the production
// FetchFunc; tests inject their own.
func GitHubLatest(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return fetchLatest(ctx, http.DefaultClient, releasesAPI)
}

// fetchLatest GETs url and returns its "tag_name". A non-200 yields an empty
// string (not an error) so a flaky/rate-limited check never bubbles up.
func fetchLatest(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.TagName, nil
}

// Suppressed reports whether the update check is disabled by environment.
func Suppressed() bool {
	return os.Getenv("MODJO_NO_UPDATE_NOTIFIER") != ""
}
