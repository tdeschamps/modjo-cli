// Package doctor implements `modjo doctor` — connectivity and credential
// diagnostics (mirrors `sentry-cli info` / `gh status`).
package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmd/version"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/config"
)

// NewCmdDoctor returns the doctor command.
func NewCmdDoctor(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Check connectivity, credentials, and configuration",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			io := f.IOStreams
			ok := func(b bool) string {
				if b {
					return io.Green("✓")
				}
				return io.Red("✗")
			}

			fmt.Fprintf(io.Out, "%s %s\n", io.Bold("modjo"), version.String())

			cfgPath, _ := config.DefaultPath()
			fmt.Fprintf(io.Out, "%s config path: %s\n", io.Cyan("•"), cfgPath)

			profile, _ := f.ActiveProfile()
			fmt.Fprintf(io.Out, "%s active profile: %s\n", io.Cyan("•"), profile)

			r, err := f.Resolver()
			if err != nil {
				return err
			}
			baseURL := r.Resolve("base_url", "", cmdutil.OSLookup)
			mcpURL := r.Resolve("mcp_url", "", cmdutil.OSLookup)

			// Credential present?
			store, err := f.CredentialStore()
			hasCred := false
			if err == nil {
				if _, gerr := store.Get(profile); gerr == nil {
					hasCred = true
				}
			}
			fmt.Fprintf(io.Out, "%s credential stored: %s\n", ok(hasCred), boolText(hasCred))

			// Probe both endpoints behind a spinner (no-op when piped/--quiet).
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			sp := io.NewSpinner("Probing endpoints…")
			sp.Start()

			// REST reachability via /me.
			client, err := f.APIClient()
			restOK := false
			var restErr error
			if err == nil {
				if _, restErr = client.Me(ctx); restErr == nil {
					restOK = true
				}
			}

			// MCP reachability via tools/list.
			mcpOK := false
			var mcpErr error
			if mc, merr := f.MCPClient(); merr == nil {
				if _, mcpErr = mc.Tools(ctx); mcpErr == nil {
					mcpOK = true
				}
			}
			sp.Stop()

			fmt.Fprintf(io.Out, "%s REST %s\n", ok(restOK), baseURL)
			if restErr != nil {
				fmt.Fprintf(io.Out, "    %s\n", io.Gray(restErr.Error()))
			}
			fmt.Fprintf(io.Out, "%s MCP  %s\n", ok(mcpOK), mcpURL)
			if mcpErr != nil {
				fmt.Fprintf(io.Out, "    %s\n", io.Gray(mcpErr.Error()))
			}

			if !restOK {
				return cmdutil.NewSilentError(cmdutil.ExitError, fmt.Errorf("REST endpoint unreachable or unauthenticated"))
			}
			return nil
		},
	}
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
