// Package users implements `modjo users`: list, get, create, delete. Writes go
// through the REST v2 management endpoints (the MCP is read-only) and honor
// --dry-run and --yes.
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
	cmd.AddCommand(newListCmd(f), newGetCmd(f), newCreateCmd(f), newDeleteCmd(f))
	return cmd
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
