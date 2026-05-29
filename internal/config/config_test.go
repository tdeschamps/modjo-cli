package config

import (
	"path/filepath"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "default",
		Profiles: map[string]*Profile{
			"default": {Output: "table", BaseURL: "https://profile.example/v2"},
		},
	}

	// profile value used when no flag/env
	r := Resolver{Config: cfg, Profile: "default"}
	if got := r.Resolve("output", "", func(string) (string, bool) { return "", false }); got != "table" {
		t.Errorf("profile precedence: got %q want table", got)
	}

	// env beats profile
	env := func(k string) (string, bool) {
		if k == "MODJO_OUTPUT" {
			return "json", true
		}
		return "", false
	}
	if got := r.Resolve("output", "", env); got != "json" {
		t.Errorf("env precedence: got %q want json", got)
	}

	// flag beats env and profile
	if got := r.Resolve("output", "csv", env); got != "csv" {
		t.Errorf("flag precedence: got %q want csv", got)
	}

	// built-in default when nothing set
	r2 := Resolver{Config: &Config{Profiles: map[string]*Profile{"default": {}}}, Profile: "default"}
	if got := r2.Resolve("base_url", "", func(string) (string, bool) { return "", false }); got != DefaultBaseURL {
		t.Errorf("default precedence: got %q want %q", got, DefaultBaseURL)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := New()
	cfg.ActiveProfile = "work"
	cfg.Profiles["work"] = &Profile{Workspace: "acme-eu", Output: "json", DefaultLimit: 25}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveProfile != "work" {
		t.Errorf("active profile: got %q", got.ActiveProfile)
	}
	p := got.Profiles["work"]
	if p == nil || p.Workspace != "acme-eu" || p.Output != "json" || p.DefaultLimit != 25 {
		t.Errorf("profile round-trip mismatch: %+v", p)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if got.ActiveProfile != "default" {
		t.Errorf("want default active profile, got %q", got.ActiveProfile)
	}
}
