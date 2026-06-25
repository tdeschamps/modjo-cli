// Package teams implements `modjo teams`: list, get, create, update, delete,
// and members. Writes go through the REST v2 management endpoints (the MCP is
// read-only) and honor --dry-run and --yes.
package teams

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdTeams returns the teams command group.
func NewCmdTeams(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "teams <command>",
		Short:   "List and manage teams",
		GroupID: "mgmt",
	}
	cmd.AddCommand(
		newListCmd(f),
		newGetCmd(f),
		newCreateCmd(f),
		newUpdateCmd(f),
		newDeleteCmd(f),
		newMembersCmd(f),
	)
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

func memberFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.TeamMember).ID.String() }},
		{Name: "EMAIL", Extract: func(v any) string { return v.(api.TeamMember).Email }},
		{Name: "FIRST", Extract: func(v any) string { return v.(api.TeamMember).FirstName }},
		{Name: "LAST", Extract: func(v any) string { return v.(api.TeamMember).LastName }},
		{Name: "ROLE", Extract: func(v any) string { return v.(api.TeamMember).Role }},
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

func newCreateCmd(f *cmdutil.Factory) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a team",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return cmdutil.NewUsageError(fmt.Errorf("--name is required"))
			}
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would create team %q\n", name)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			team, err := client.CreateTeam(cmd.Context(), api.CreateTeamInput{Name: name})
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Created team %s (id %s)\n", f.IOStreams.Green("✓"), team.Name, team.ID.String())
			return cmdutil.RenderSlice(f, []api.Team{team}, teamFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name of the new team (required)")
	return cmd
}

func newUpdateCmd(f *cmdutil.Factory) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "update <id> --name <name>",
		Short: "Rename a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return cmdutil.NewUsageError(fmt.Errorf("--name is required"))
			}
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would rename team %s to %q\n", id, name)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			team, err := client.UpdateTeam(cmd.Context(), id, api.UpdateTeamInput{Name: name})
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Updated team %s\n", f.IOStreams.Green("✓"), id)
			return cmdutil.RenderSlice(f, []api.Team{team}, teamFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name for the team (required)")
	return cmd
}

func newDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would delete team %s\n", id)
				return nil
			}
			if !f.Flags.Yes {
				ok, err := cmdutil.Confirm(f.IOStreams, fmt.Sprintf("Delete team %s?", id), false)
				if err != nil {
					return err
				}
				if !ok {
					return cmdutil.NewSilentError(cmdutil.ExitOK, fmt.Errorf("aborted"))
				}
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteTeam(cmd.Context(), id); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Deleted team %s\n", f.IOStreams.Green("✓"), id)
			return nil
		},
	}
}

func newMembersCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "members <id>",
		Short: "List the members of a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			seq := client.TeamMembers(cmd.Context(), args[0], api.PageFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, memberFields(), "members")
		},
	}
}
