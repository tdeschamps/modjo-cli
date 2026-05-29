// Package config handles the layered configuration model described in the
// product spec: a TOML file holding one or more named profiles, with runtime
// resolution following the precedence flag → env → profile → built-in default.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Built-in defaults and endpoint constants (product spec §4.1, §5).
const (
	DefaultBaseURL     = "https://api.modjo.ai/v2"
	DefaultMCPURL      = "https://api.mcp.modjo.ai/v1/mcp"
	DefaultOutput      = "table"
	DefaultColor       = "auto"
	DefaultLanguage    = "en"
	DefaultLimit       = 50
	DefaultProfileName = "default"
	DefaultMaxRetries  = 3
	IntegrationsURL    = "https://app.modjo.ai/settings/integrations/modjo_api_key"
	DeviceAuthURL      = "https://app.modjo.ai/device"
)

// Profile is a single named workspace configuration.
type Profile struct {
	Workspace    string `toml:"workspace,omitempty"`
	AuthMethod   string `toml:"auth_method,omitempty"` // api_key | oauth
	BaseURL      string `toml:"base_url,omitempty"`
	MCPURL       string `toml:"mcp_url,omitempty"`
	Output       string `toml:"output,omitempty"`
	Color        string `toml:"color,omitempty"`
	Language     string `toml:"language,omitempty"`
	DefaultLimit int    `toml:"default_limit,omitempty"`
}

// Config is the top-level config document.
type Config struct {
	ActiveProfile string              `toml:"active_profile"`
	Profiles      map[string]*Profile `toml:"profiles"`
}

// New returns a Config containing an empty default profile.
func New() *Config {
	return &Config{
		ActiveProfile: DefaultProfileName,
		Profiles:      map[string]*Profile{DefaultProfileName: {}},
	}
}

// DefaultPath returns the XDG-respecting config path for the current OS.
func DefaultPath() (string, error) {
	if p := os.Getenv("MODJO_CONFIG"); p != "" {
		return p, nil
	}
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "modjo", "config.toml"), nil
}

// Load reads the config file at path. A missing file yields a fresh default
// Config (not an error) so first-run is friction-free.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = DefaultProfileName
	}
	if _, ok := cfg.Profiles[cfg.ActiveProfile]; !ok {
		cfg.Profiles[cfg.ActiveProfile] = &Profile{}
	}
	return cfg, nil
}

// Save writes the config file atomically with 0600 perms (it may reference
// secret storage and should never be world-readable).
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ProfileOrDefault returns the named profile, creating an empty one if absent.
func (c *Config) ProfileOrDefault(name string) *Profile {
	if name == "" {
		name = c.ActiveProfile
	}
	if name == "" {
		name = DefaultProfileName
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	p, ok := c.Profiles[name]
	if !ok {
		p = &Profile{}
		c.Profiles[name] = p
	}
	return p
}

// Resolver resolves a single setting following flag → env → profile → default.
type Resolver struct {
	Config  *Config
	Profile string
}

// EnvLookup matches os.LookupEnv's signature so tests can inject fake env.
type EnvLookup func(string) (string, bool)

var defaults = map[string]string{
	"base_url":      DefaultBaseURL,
	"mcp_url":       DefaultMCPURL,
	"output":        DefaultOutput,
	"color":         DefaultColor,
	"language":      DefaultLanguage,
	"default_limit": strconv.Itoa(DefaultLimit),
}

var envKeys = map[string]string{
	"base_url": "MODJO_BASE_URL",
	"mcp_url":  "MODJO_MCP_URL",
	"output":   "MODJO_OUTPUT",
	"language": "MODJO_LANGUAGE",
}

// Resolve returns the effective string value for key. flagVal is the value the
// user passed on the command line ("" means unset). env looks up environment
// variables.
func (r Resolver) Resolve(key, flagVal string, env EnvLookup) string {
	if strings.TrimSpace(flagVal) != "" {
		return flagVal
	}
	if envKey, ok := envKeys[key]; ok {
		if v, ok := env(envKey); ok && v != "" {
			return v
		}
	}
	if r.Config != nil {
		if p, ok := r.Config.Profiles[r.Profile]; ok && p != nil {
			if v := profileField(p, key); v != "" {
				return v
			}
		}
	}
	return defaults[key]
}

func profileField(p *Profile, key string) string {
	switch key {
	case "workspace":
		return p.Workspace
	case "auth_method":
		return p.AuthMethod
	case "base_url":
		return p.BaseURL
	case "mcp_url":
		return p.MCPURL
	case "output":
		return p.Output
	case "color":
		return p.Color
	case "language":
		return p.Language
	case "default_limit":
		if p.DefaultLimit > 0 {
			return strconv.Itoa(p.DefaultLimit)
		}
	}
	return ""
}
