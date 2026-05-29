package config

import (
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	cfgpkg "github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func TestSetFieldAllKeys(t *testing.T) {
	p := &cfgpkg.Profile{}
	for key, val := range map[string]string{
		"workspace": "w", "auth_method": "oauth", "base_url": "b", "mcp_url": "m",
		"output": "json", "color": "never", "language": "fr", "default_limit": "42",
	} {
		if err := setField(p, key, val); err != nil {
			t.Errorf("setField(%q): %v", key, err)
		}
	}
	if p.Workspace != "w" || p.DefaultLimit != 42 || p.Color != "never" {
		t.Errorf("fields not set: %+v", p)
	}
	if err := setField(p, "default_limit", "notint"); err == nil {
		t.Error("non-int default_limit should error")
	}
}

func TestValidKey(t *testing.T) {
	if !validKey("output") || validKey("bogus") {
		t.Error("validKey")
	}
}

func TestConfigEditUsesEditor(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml"}
	// Use a harmless editor that exits 0.
	t.Setenv("EDITOR", "true")
	cmd := newEditCmd(f)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config edit: %v", err)
	}
}

func TestConfigEditDefaultEditor(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}}
	// Empty EDITOR falls back to "vi"; point at a missing path via MODJO_CONFIG
	// and a non-existent editor so Run errors fast (covers the error return).
	t.Setenv("EDITOR", "/nonexistent-editor-xyz")
	t.Setenv("MODJO_CONFIG", t.TempDir()+"/c.toml")
	cmd := newEditCmd(f)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error from missing editor")
	}
}
