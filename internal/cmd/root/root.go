// Package root assembles the top-level `modjo` command: it registers global
// flags, wires the command tree, and configures TTY/color behavior before any
// subcommand runs.
package root

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmd/accounts"
	"github.com/tdeschamps/modjo-cli/internal/cmd/agents"
	apicmd "github.com/tdeschamps/modjo-cli/internal/cmd/api"
	"github.com/tdeschamps/modjo-cli/internal/cmd/ask"
	authcmd "github.com/tdeschamps/modjo-cli/internal/cmd/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmd/calls"
	"github.com/tdeschamps/modjo-cli/internal/cmd/completion"
	configcmd "github.com/tdeschamps/modjo-cli/internal/cmd/config"
	"github.com/tdeschamps/modjo-cli/internal/cmd/contacts"
	"github.com/tdeschamps/modjo-cli/internal/cmd/deals"
	"github.com/tdeschamps/modjo-cli/internal/cmd/docs"
	"github.com/tdeschamps/modjo-cli/internal/cmd/doctor"
	"github.com/tdeschamps/modjo-cli/internal/cmd/emails"
	mcpcmd "github.com/tdeschamps/modjo-cli/internal/cmd/mcp"
	"github.com/tdeschamps/modjo-cli/internal/cmd/profiles"
	"github.com/tdeschamps/modjo-cli/internal/cmd/teams"
	"github.com/tdeschamps/modjo-cli/internal/cmd/update"
	"github.com/tdeschamps/modjo-cli/internal/cmd/users"
	"github.com/tdeschamps/modjo-cli/internal/cmd/version"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

// NewCmdRoot builds the root command and its subtree using the given factory.
func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	flags := f.Flags
	var showVersion bool

	cmd := &cobra.Command{
		Use:   "modjo <command> <subcommand> [flags]",
		Short: "Modjo CLI — script the Modjo API and drive it with natural language",
		Long: `modjo wraps the Modjo REST API v2 and the Modjo MCP server behind one
consistent, scriptable, agent-friendly interface.

Output adapts to context: pretty tables in a terminal, JSON when piped.
See 'modjo <command> --help' for details on any command.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				cmd.Println(version.String())
				return nil
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyPresentation(f)
			return nil
		},
	}

	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	cmd.SetIn(f.IOStreams.In)

	pf := cmd.PersistentFlags()
	pf.StringVar(&flags.Profile, "profile", "", "Use a specific profile")
	pf.StringVar(&flags.APIKey, "api-key", "", "One-off API key override")
	pf.StringVar(&flags.Token, "token", "", "One-off bearer token override")
	pf.StringVarP(&flags.Output, "output", "o", "", "Output format: table|json|csv|tsv|yaml")
	pf.BoolVar(&flags.JSON, "json", false, "Shorthand for --output json")
	pf.StringVar(&flags.JQ, "jq", "", "Apply a jq-style filter to JSON output")
	pf.StringSliceVar(&flags.Columns, "columns", nil, "Pick/order columns (table/csv)")
	pf.IntVar(&flags.Limit, "limit", 0, "Cap results (default: profile default_limit)")
	pf.BoolVar(&flags.All, "all", false, "Auto-paginate through every page")
	pf.BoolVar(&flags.NoColor, "no-color", false, "Disable color")
	pf.StringVar(&flags.Color, "color", "", "Color: auto|always|never")
	pf.BoolVarP(&flags.Quiet, "quiet", "q", false, "Suppress non-essential output")
	pf.BoolVar(&flags.Debug, "debug", false, "Verbose request/response logging")
	pf.BoolVar(&flags.DebugUnsafe, "debug-unsafe", false, "Like --debug but shows the auth header")
	pf.BoolVar(&flags.DryRun, "dry-run", false, "Show what would happen, make no changes")
	pf.BoolVarP(&flags.Yes, "yes", "y", false, "Skip confirmation prompts")
	pf.BoolVar(&flags.Insecure, "insecure", false, "Skip TLS verification (debugging only)")
	pf.IntVar(&flags.MaxRetries, "max-retries", 0, "Max retries on 429/5xx")

	// --version on the root prints our richer string.
	pf.BoolVar(&showVersion, "version", false, "Show version information")
	cmd.SetVersionTemplate(version.String() + "\n")

	cmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Core commands:"},
		&cobra.Group{ID: "ai", Title: "AI & interop:"},
		&cobra.Group{ID: "mgmt", Title: "Management:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
	)

	// Resource commands.
	cmd.AddCommand(
		authcmd.NewCmdAuth(f),
		configcmd.NewCmdConfig(f),
		profiles.NewCmdProfiles(f),
		calls.NewCmdCalls(f),
		deals.NewCmdDeals(f),
		accounts.NewCmdAccounts(f),
		contacts.NewCmdContacts(f),
		emails.NewCmdEmails(f),
		users.NewCmdUsers(f),
		teams.NewCmdTeams(f),
		agents.NewCmdAgents(f),
		ask.NewCmdAsk(f),
		mcpcmd.NewCmdMCP(f),
		apicmd.NewCmdAPI(f),
		doctor.NewCmdDoctor(f),
		completion.NewCmdCompletion(f),
		docs.NewCmdDocs(f),
		update.NewCmdUpdate(f),
		version.NewCmdVersion(f),
	)

	return cmd
}

// applyPresentation resolves color/TTY presentation from flags before commands run.
func applyPresentation(f *cmdutil.Factory) {
	io := f.IOStreams
	switch f.Flags.Color {
	case "always":
		io.SetColorEnabled(true)
	case "never":
		io.SetColorEnabled(false)
	}
	if f.Flags.NoColor {
		io.SetColorEnabled(false)
	}
	if f.Flags.Quiet {
		io.SetNeverPrompt(true)
	}
}
