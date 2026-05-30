// Package cmdutil holds the glue between Cobra commands and the rest of the
// system: the Factory (dependency injection), global flag state, exit-code
// mapping, and small command helpers. Commands stay thin by leaning on this.
package cmdutil

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/httpclient"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/mcp"
	"github.com/tdeschamps/modjo-cli/internal/output"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

// GlobalFlags holds every flag available on all commands (product spec §6).
type GlobalFlags struct {
	Profile     string
	APIKey      string
	Token       string
	Output      string
	JSON        bool
	JQ          string
	Columns     []string
	Limit       int
	All         bool
	NoColor     bool
	Color       string
	Quiet       bool
	Debug       bool
	DebugUnsafe bool
	DryRun      bool
	Yes         bool
	Insecure    bool
	MaxRetries  int
}

// Factory builds the dependencies a command needs, lazily and consistently.
type Factory struct {
	IOStreams *iostreams.IOStreams
	Flags     *GlobalFlags
	Clock     text.Clock

	// ConfigPath overrides where config is loaded from (tests).
	ConfigPath string
	// CredStore overrides the credential store (tests).
	CredStore auth.Store
	// Transport overrides the base HTTP transport (tests).
	Transport http.RoundTripper

	cfg *config.Config
}

// credentialsPath returns the credentials file path next to the config file.
func credentialsPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "credentials")
}

// New returns a Factory wired to the real environment.
func New(io *iostreams.IOStreams, flags *GlobalFlags) *Factory {
	return &Factory{IOStreams: io, Flags: flags, Clock: text.SystemClock()}
}

// Config loads (and caches) the configuration.
func (f *Factory) Config() (*config.Config, error) {
	if f.cfg != nil {
		return f.cfg, nil
	}
	path := f.ConfigPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	f.cfg = cfg
	return cfg, nil
}

// ActiveProfile returns the effective profile name (flag wins over config).
func (f *Factory) ActiveProfile() (string, error) {
	if p := f.flagOrEnvProfile(); p != "" {
		return p, nil
	}
	cfg, err := f.Config()
	if err != nil {
		return "", err
	}
	return cfg.ActiveProfile, nil
}

// flagOrEnvProfile returns the profile from --profile or MODJO_PROFILE, or "".
func (f *Factory) flagOrEnvProfile() string {
	if f.Flags.Profile != "" {
		return f.Flags.Profile
	}
	return os.Getenv("MODJO_PROFILE")
}

// activeProfileFrom resolves the active profile against an already-loaded config
// (no error path, since the config is in hand).
func (f *Factory) activeProfileFrom(cfg *config.Config) string {
	if p := f.flagOrEnvProfile(); p != "" {
		return p
	}
	return cfg.ActiveProfile
}

// Resolver returns a config.Resolver bound to the active profile.
func (f *Factory) Resolver() (config.Resolver, error) {
	cfg, err := f.Config()
	if err != nil {
		return config.Resolver{}, err
	}
	return config.Resolver{Config: cfg, Profile: f.activeProfileFrom(cfg)}, nil
}

// resolve is a convenience to resolve a single config key.
func (f *Factory) resolve(key, flagVal string) (string, error) {
	r, err := f.Resolver()
	if err != nil {
		return "", err
	}
	return r.Resolve(key, flagVal, os.LookupEnv), nil
}

// CredentialStore returns the configured credential store, defaulting to a
// keychain-backed store with a file fallback.
func (f *Factory) CredentialStore() (auth.Store, error) {
	if f.CredStore != nil {
		return f.CredStore, nil
	}
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	fallback := credentialsPath(cfgPath)
	store := auth.NewKeyringStore(fallback)
	f.CredStore = store
	return store, nil
}

// TokenSource resolves the bearer token following flag → env → stored cred.
// The stored-credential lookup (which may hit the OS keychain) is resolved at
// most once per returned closure via sync.Once, so a paginated `--all` export
// doesn't query the keychain once per HTTP request. Flag/env overrides are
// re-read each call so they always win and stay cheap.
func (f *Factory) TokenSource() func() (string, error) {
	var (
		once   sync.Once
		stored string
		stErr  error
	)
	return func() (string, error) {
		if f.Flags.APIKey != "" {
			return f.Flags.APIKey, nil
		}
		if f.Flags.Token != "" {
			return f.Flags.Token, nil
		}
		if v := os.Getenv("MODJO_API_KEY"); v != "" {
			return v, nil
		}
		if v := os.Getenv("MODJO_TOKEN"); v != "" {
			return v, nil
		}
		once.Do(func() {
			cfg, err := f.Config()
			if err != nil {
				stErr = err
				return
			}
			store, err := f.CredentialStore()
			if err != nil {
				stErr = err
				return
			}
			cred, err := store.Get(f.activeProfileFrom(cfg))
			if err != nil {
				stErr = ErrNotAuthenticated
				return
			}
			stored = cred.Token
		})
		return stored, stErr
	}
}

// HTTPClient builds the RoundTripper chain (auth → retry → logging).
func (f *Factory) httpClient() *http.Client {
	maxRetries := f.Flags.MaxRetries
	if maxRetries == 0 {
		maxRetries = config.DefaultMaxRetries
	}
	return httpclient.New(httpclient.Options{
		Token:       f.TokenSource(),
		MaxRetries:  maxRetries,
		Debug:       f.Flags.Debug,
		DebugUnsafe: f.Flags.DebugUnsafe,
		DebugOut:    f.IOStreams.ErrOut,
		Insecure:    f.Flags.Insecure,
		Base:        f.Transport,
	})
}

// APIClient builds a REST client for the active profile.
func (f *Factory) APIClient() (*api.Client, error) {
	baseURL, err := f.resolve("base_url", "")
	if err != nil {
		return nil, err
	}
	return api.New(api.Options{BaseURL: baseURL, HTTPClient: f.httpClient()}), nil
}

// MCPClient builds an MCP client for the active profile.
func (f *Factory) MCPClient() (*mcp.Client, error) {
	endpoint, err := f.resolve("mcp_url", "")
	if err != nil {
		return nil, err
	}
	return mcp.New(mcp.Options{Endpoint: endpoint, HTTPClient: f.httpClient()}), nil
}

// Printer builds an output.Printer honoring -o/--json/--jq/--columns/color.
func (f *Factory) Printer() (*output.Printer, error) {
	format, err := f.OutputFormat()
	if err != nil {
		return nil, err
	}
	return &output.Printer{
		Out:          f.IOStreams.Out,
		Format:       format,
		Columns:      f.Flags.Columns,
		JQ:           f.Flags.JQ,
		ColorEnabled: f.IOStreams.ColorEnabled(),
	}, nil
}

// OutputFormat computes the effective output format.
func (f *Factory) OutputFormat() (output.Format, error) {
	if f.Flags.JSON {
		return output.FormatJSON, nil
	}
	raw, err := f.resolve("output", f.Flags.Output)
	if err != nil {
		return "", err
	}
	// In a non-TTY, default to JSON unless the user asked for something.
	if f.Flags.Output == "" && raw == config.DefaultOutput && !f.IOStreams.IsStdoutTTY() {
		return output.FormatJSON, nil
	}
	format, err := output.ParseFormat(raw)
	if err != nil {
		return "", err
	}
	// --jq only applies to structured output. If the user asked to filter but
	// the effective format is tabular, promote to JSON so the filter isn't
	// silently dropped (yaml is left alone — gojq results render fine as YAML).
	if f.Flags.JQ != "" && format != output.FormatJSON && format != output.FormatYAML {
		return output.FormatJSON, nil
	}
	return format, nil
}

// EffectiveLimit returns the result cap: --all → 0 (unbounded), else --limit or
// the profile default.
func (f *Factory) EffectiveLimit() (int, error) {
	if f.Flags.All {
		return 0, nil
	}
	if f.Flags.Limit > 0 {
		return f.Flags.Limit, nil
	}
	raw, err := f.resolve("default_limit", "")
	if err != nil {
		return 0, err
	}
	n := config.DefaultLimit
	_, _ = fmt.Sscanf(raw, "%d", &n)
	return n, nil
}
