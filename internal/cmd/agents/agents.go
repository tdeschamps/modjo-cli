// Package agents implements `modjo agents`: list, get. Agents (native + custom)
// can be passed to `modjo ask --agent`.
package agents

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdAgents returns the agents command group.
func NewCmdAgents(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agents <command>",
		Short:   "List and inspect analysis agents",
		GroupID: "ai",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f))
	return cmd
}

func agentFields() []output.Field {
	return []output.Field{
		{Name: "UUID", Extract: func(v any) string { return v.(api.Agent).UUID }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Agent).Name }},
		{Name: "ORIGIN", Extract: func(v any) string { return v.(api.Agent).Origin }},
		{Name: "DESCRIPTION", Extract: func(v any) string { return v.(api.Agent).Description }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var search, origin string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List native and custom agents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			seq := client.Agents(cmd.Context(), api.AgentFilter{Search: search, Origin: origin, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, agentFields())
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Filter by search term")
	cmd.Flags().StringVar(&origin, "origin", "", "Filter by origin (modjo|user)")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <uuid>...",
		Short: "Get one or more agents",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetAgent, agentFields())
		},
	}
}
