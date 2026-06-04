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
		{Name: "CREATED", Extract: func(v any) string { return v.(api.Team).CreatedOn }},
		{Name: "MODIFIED", Extract: func(v any) string { return v.(api.Team).ModifiedOn }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List teams",
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
			seq := client.Teams(cmd.Context(), api.TeamFilter{Name: name, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, teamFields(), "teams")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by team name")
	return cmd
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
