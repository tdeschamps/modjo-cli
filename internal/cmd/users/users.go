// Package users implements `modjo users`: list, get, create, update, delete,
// add-team, and remove-team. Writes go through the REST v2 management endpoints
// (the MCP is read-only) and honor --dry-run and --yes.
package users

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdUsers returns the users command group.
func NewCmdUsers(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "users <command>",
		Short:   "List and manage users",
		GroupID: "mgmt",
	}
	cmd.AddCommand(
		newListCmd(f),
		newGetCmd(f),
		newCreateCmd(f),
		newUpdateCmd(f),
		newDeleteCmd(f),
		newAddTeamCmd(f),
		newRemoveTeamCmd(f),
	)
	return cmd
}

func userTeamFields() []output.Field {
	return []output.Field{
		{Name: "USER", Extract: func(v any) string { return v.(api.UserTeam).UserID.String() }},
		{Name: "TEAM", Extract: func(v any) string { return v.(api.UserTeam).TeamID.String() }},
	}
}

func userFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.User).ID.String() }},
		{Name: "EMAIL", Extract: func(v any) string { return v.(api.User).Email }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.User).Name }},
		{Name: "ROLE", Extract: func(v any) string { return v.(api.User).Role }},
		{Name: "DEPARTMENT", Extract: func(v any) string { return v.(api.User).Department }},
		{Name: "TITLE", Extract: func(v any) string { return v.(api.User).Title }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
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
			seq := client.Users(cmd.Context(), api.UserFilter{Email: email, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, userFields(), "users")
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Filter by email (exact match)")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>...",
		Short: "Get one or more users",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetUser, userFields())
		},
	}
}

func newCreateCmd(f *cmdutil.Factory) *cobra.Command {
	var email, firstName, lastName, role, jobTitle, jobDepartment, phone, timezone string
	cmd := &cobra.Command{
		Use:   "create --first-name <first> --last-name <last> --email <email>",
		Short: "Create a user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if firstName == "" || lastName == "" || email == "" {
				return cmdutil.NewUsageError(fmt.Errorf("--first-name, --last-name and --email are required"))
			}
			in := api.CreateUserInput{
				Email:         email,
				FirstName:     firstName,
				LastName:      lastName,
				Role:          role,
				JobTitle:      jobTitle,
				JobDepartment: jobDepartment,
				PhoneNumber:   phone,
				Timezone:      timezone,
			}
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would create user %s %s <%s> (role=%q)\n", firstName, lastName, email, role)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			u, err := client.CreateUser(cmd.Context(), in)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Created user %s (id %s)\n", f.IOStreams.Green("✓"), u.Email, u.ID.String())
			return cmdutil.RenderSlice(f, []api.User{u}, userFields())
		},
	}
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name of the new user (required)")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name of the new user (required)")
	cmd.Flags().StringVar(&email, "email", "", "Email of the new user (required)")
	cmd.Flags().StringVar(&role, "role", "", "Role for the new user")
	cmd.Flags().StringVar(&jobTitle, "job-title", "", "Job title for the new user")
	cmd.Flags().StringVar(&jobDepartment, "job-department", "", "Job department for the new user")
	cmd.Flags().StringVar(&phone, "phone", "", "Phone number for the new user")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone for the new user")
	return cmd
}

func newUpdateCmd(f *cmdutil.Factory) *cobra.Command {
	var email, firstName, lastName, role, jobTitle, jobDepartment, phone, timezone string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build the body from flags the user actually set, so a provided
			// flag is always sent (even when its value is empty, to clear a
			// field) and an unset flag is omitted. Pairs each flag name with the
			// struct field it fills.
			var in api.UpdateUserInput
			setIfChanged := func(flag string, dst **string, val string) {
				if cmd.Flags().Changed(flag) {
					*dst = api.Ptr(val)
				}
			}
			setIfChanged("email", &in.Email, email)
			setIfChanged("first-name", &in.FirstName, firstName)
			setIfChanged("last-name", &in.LastName, lastName)
			setIfChanged("phone", &in.PhoneNumber, phone)
			setIfChanged("job-title", &in.JobTitle, jobTitle)
			setIfChanged("job-department", &in.JobDepartment, jobDepartment)
			setIfChanged("role", &in.Role, role)
			setIfChanged("timezone", &in.Timezone, timezone)
			if in == (api.UpdateUserInput{}) {
				return cmdutil.NewUsageError(fmt.Errorf("at least one field flag is required"))
			}
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would update user %s\n", id)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			u, err := client.UpdateUser(cmd.Context(), id, in)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Updated user %s\n", f.IOStreams.Green("✓"), id)
			return cmdutil.RenderSlice(f, []api.User{u}, userFields())
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "New email")
	cmd.Flags().StringVar(&firstName, "first-name", "", "New first name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "New last name")
	cmd.Flags().StringVar(&phone, "phone", "", "New phone number")
	cmd.Flags().StringVar(&jobTitle, "job-title", "", "New job title")
	cmd.Flags().StringVar(&jobDepartment, "job-department", "", "New job department")
	cmd.Flags().StringVar(&role, "role", "", "New role")
	cmd.Flags().StringVar(&timezone, "timezone", "", "New timezone")
	return cmd
}

func newAddTeamCmd(f *cmdutil.Factory) *cobra.Command {
	var team int
	cmd := &cobra.Command{
		Use:   "add-team <id> --team <teamId>",
		Short: "Add a user to a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("team") {
				return cmdutil.NewUsageError(fmt.Errorf("--team is required"))
			}
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would add user %s to team %d\n", id, team)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			ut, err := client.AddUserTeam(cmd.Context(), id, team)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Added user %s to team %d\n", f.IOStreams.Green("✓"), id, team)
			return cmdutil.RenderSlice(f, []api.UserTeam{ut}, userTeamFields())
		},
	}
	cmd.Flags().IntVar(&team, "team", 0, "ID of the team to add the user to (required)")
	return cmd
}

func newRemoveTeamCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-team <id> <teamId>",
		Short: "Remove a user from a team",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, teamID := args[0], args[1]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would remove user %s from team %s\n", id, teamID)
				return nil
			}
			if !f.Flags.Yes {
				ok, err := cmdutil.Confirm(f.IOStreams, fmt.Sprintf("Remove user %s from team %s?", id, teamID), false)
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
			if err := client.RemoveUserTeam(cmd.Context(), id, teamID); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Removed user %s from team %s\n", f.IOStreams.Green("✓"), id, teamID)
			return nil
		},
	}
}

func newDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would delete user %s\n", id)
				return nil
			}
			if !f.Flags.Yes {
				ok, err := cmdutil.Confirm(f.IOStreams, fmt.Sprintf("Delete user %s?", id), false)
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
			if err := client.DeleteUser(cmd.Context(), id); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Deleted user %s\n", f.IOStreams.Green("✓"), id)
			return nil
		},
	}
}
