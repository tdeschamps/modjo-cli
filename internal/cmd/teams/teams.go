// Package teams implements `modjo teams`: list, get.
package teams

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdTeams returns the teams command group.
func NewCmdTeams(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "teams <command>",
		Short:   "List and inspect teams",
		GroupID: "mgmt",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f))
	return cmd
}

func teamFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Team).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Team).Name }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List teams",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Teams(cmd.Context()), teamFields(), "teams")
		},
	}
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>...",
		Short: "Get one or more teams",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetTeam, teamFields())
		},
	}
}
