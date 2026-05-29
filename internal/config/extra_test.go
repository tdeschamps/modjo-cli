package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPathBranches(t *testing.T) {
	t.Setenv("MODJO_CONFIG", "/custom/path.toml")
	if p, _ := DefaultPath(); p != "/custom/path.toml" {
		t.Errorf("MODJO_CONFIG override = %q", p)
	}

	t.Setenv("MODJO_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if runtime.GOOS != "windows" {
		p, _ := DefaultPath()
		if p != filepath.Join("/xdg", "modjo", "config.toml") {
			t.Errorf("XDG path = %q", p)
		}
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := DefaultPath()
	if err != nil || p == "" {
		t.Errorf("HOME path = %q, %v", p, err)
	}
}

func TestDefaultPathError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME semantics differ on Windows")
	}
	t.Setenv("MODJO_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := DefaultPath(); err == nil {
		t.Error("expected error when home is undiscoverable")
	}
}

func TestLoadBadActiveProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.toml")
	// Active profile points at a profile that isn't defined.
	_ = os.WriteFile(path, []byte("active_profile = \"ghost\"\n"), 0o600)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profiles["ghost"]; !ok {
		t.Error("Load should materialize the missing active profile")
	}
}

func TestLoadEmptyActiveProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.toml")
	_ = os.WriteFile(path, []byte("[profiles.default]\noutput = \"json\"\n"), 0o600)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != DefaultProfileName {
		t.Errorf("empty active profile should default, got %q", cfg.ActiveProfile)
	}
}

func TestLoadReadErrorOnDir(t *testing.T) {
	// Reading a directory as a file yields a non-NotExist error.
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("expected error reading a directory as config")
	}
}

func TestSaveMkdirError(t *testing.T) {
	// A path whose parent is a file (not a dir) makes MkdirAll fail.
	dir := t.TempDir()
	file := filepath.Join(dir, "afile")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	badPath := filepath.Join(file, "sub", "config.toml")
	if err := Save(badPath, New()); err == nil {
		t.Error("expected mkdir error when parent is a file")
	}
}

func TestProfileOrDefault(t *testing.T) {
	c := &Config{ActiveProfile: "p", Profiles: map[string]*Profile{}}
	if got := c.ProfileOrDefault(""); got == nil {
		t.Error("empty name should resolve to active profile")
	}
	c2 := &Config{Profiles: nil}
	if got := c2.ProfileOrDefault(""); got == nil {
		t.Error("nil profiles + empty active should still return a profile")
	}
	if got := c.ProfileOrDefault("named"); got == nil {
		t.Error("named profile should be created")
	}
}

func TestProfileFieldAllKeys(t *testing.T) {
	p := &Profile{
		Workspace: "w", AuthMethod: "api_key", BaseURL: "b", MCPURL: "m",
		Output: "json", Color: "always", Language: "fr", DefaultLimit: 25,
	}
	for key, want := range map[string]string{
		"workspace": "w", "auth_method": "api_key", "base_url": "b", "mcp_url": "m",
		"output": "json", "color": "always", "language": "fr", "default_limit": "25",
	} {
		if got := profileField(p, key); got != want {
			t.Errorf("profileField(%q) = %q want %q", key, got, want)
		}
	}
	if profileField(p, "unknown") != "" {
		t.Error("unknown key → empty")
	}
	if profileField(&Profile{}, "default_limit") != "" {
		t.Error("zero default_limit → empty")
	}
}
