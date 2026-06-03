// Package info implements `modjo info` — a friendly banner showing the CLI
// version, the active configuration, and auth status. On a terminal it prints
// the Modjo logo plus a readable summary; piped or with --json it emits a plain
// structured object, so it stays scriptable.
package info

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmd/version"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

// logo is the Modjo brand mark: an ASCII-density disc with the waveform bars
// carved out as negative space, beside the wordmark. It's plain text (one cyan
// wrap at render time) so it degrades cleanly to pipes, --no-color, and NO_COLOR.
//
//go:embed logo.txt
var logo string

const docsURL = "https://github.com/tdeschamps/modjo-cli#readme"

// data is the structured form of `info`, used for json/piped output.
type data struct {
	Version          string `json:"version"`
	Commit           string `json:"commit"`
	BuildDate        string `json:"buildDate"`
	Profile          string `json:"profile"`
	Workspace        string `json:"workspace,omitempty"`
	BaseURL          string `json:"baseUrl"`
	MCPURL           string `json:"mcpUrl"`
	ConfigPath       string `json:"configPath"`
	Authenticated    bool   `json:"authenticated"`
	AuthMethod       string `json:"authMethod,omitempty"`
	TokenFingerprint string `json:"tokenFingerprint,omitempty"`
	RESTReachable    *bool  `json:"restReachable,omitempty"`
	MCPReachable     *bool  `json:"mcpReachable,omitempty"`
}

// NewCmdInfo returns the info command.
func NewCmdInfo(f *cmdutil.Factory) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:     "info",
		Short:   "Show the CLI version, configuration, and status",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := gather(f)
			if err != nil {
				return err
			}

			if check {
				rest, mcp := probe(cmd.Context(), f)
				d.RESTReachable, d.MCPReachable = &rest, &mcp
			}

			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			if !format.IsInteractive() {
				p, _ := f.Printer()
				return p.PrintJSON(d)
			}
			renderHuman(f.IOStreams, d)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Also probe REST + MCP connectivity")
	return cmd
}

func gather(f *cmdutil.Factory) (data, error) {
	r, err := f.Resolver()
	if err != nil {
		return data{}, err
	}
	profile, _ := f.ActiveProfile()
	cfgPath, _ := config.DefaultPath()
	if f.ConfigPath != "" {
		cfgPath = f.ConfigPath
	}

	d := data{
		Version:    version.Version,
		Commit:     version.Commit,
		BuildDate:  version.Date,
		Profile:    profile,
		BaseURL:    r.Resolve("base_url", "", cmdutil.OSLookup),
		MCPURL:     r.Resolve("mcp_url", "", cmdutil.OSLookup),
		ConfigPath: cfgPath,
		Workspace:  r.Resolve("workspace", "", cmdutil.OSLookup),
	}

	if store, serr := f.CredentialStore(); serr == nil {
		if cred, cerr := store.Get(profile); cerr == nil {
			d.Authenticated = true
			d.AuthMethod = cred.Method
			d.TokenFingerprint = auth.Fingerprint(cred.Token)
			if d.Workspace == "" {
				d.Workspace = cred.Workspace
			}
		}
	}
	return d, nil
}

// probe checks REST and MCP reachability, showing a spinner while it waits.
func probe(ctx context.Context, f *cmdutil.Factory) (rest, mcp bool) {
	sp := f.IOStreams.NewSpinner("Checking connectivity…")
	sp.Start()
	defer sp.Stop()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if c, err := f.APIClient(); err == nil {
		_, err = c.Me(ctx)
		rest = err == nil
	}
	if c, err := f.MCPClient(); err == nil {
		_, err = c.Tools(ctx)
		mcp = err == nil
	}
	return rest, mcp
}

func renderHuman(io *iostreams.IOStreams, d data) {
	out := io.Out
	fmt.Fprintln(out, io.Cyan(logo))

	// Pad the label to a fixed width *before* colorizing so the ANSI escape
	// bytes don't throw off the alignment.
	row := func(label, value string) {
		fmt.Fprintf(out, "  %s %s\n", io.Bold(fmt.Sprintf("%-7s", label)), value)
	}
	row("Version", fmt.Sprintf("%s (%s)", d.Version, d.Commit))
	if d.Workspace != "" {
		row("Profile", fmt.Sprintf("%s (%s)", d.Profile, d.Workspace))
	} else {
		row("Profile", d.Profile)
	}
	if d.Authenticated {
		row("Auth", fmt.Sprintf("%s %s (%s)", io.SuccessIcon(), d.TokenFingerprint, d.AuthMethod))
	} else {
		row("Auth", fmt.Sprintf("%s not logged in", io.ErrorIcon()))
	}
	row("REST", d.BaseURL)
	row("MCP", d.MCPURL)
	row("Config", d.ConfigPath)

	if d.RESTReachable != nil {
		row("Check", fmt.Sprintf("REST %s   MCP %s", reach(io, *d.RESTReachable), reach(io, *d.MCPReachable)))
	}

	fmt.Fprintln(out)
	if d.Authenticated {
		fmt.Fprintf(out, "  %s Try %s or %s\n", io.Gray("›"), io.Cyan("modjo calls list"), io.Cyan("modjo ask deal <id> \"...\""))
	} else {
		fmt.Fprintf(out, "  %s Run %s to get started\n", io.Gray("›"), io.Cyan("modjo auth login"))
	}
	fmt.Fprintf(out, "  %s Docs: %s\n", io.Gray("›"), io.Hyperlink(docsURL, docsURL))
}

func reach(io *iostreams.IOStreams, ok bool) string {
	if ok {
		return io.SuccessIcon()
	}
	return io.ErrorIcon()
}
