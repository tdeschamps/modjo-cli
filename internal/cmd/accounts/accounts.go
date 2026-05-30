// Package accounts implements `modjo accounts`: list, get, open.
package accounts

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdAccounts returns the accounts command group.
func NewCmdAccounts(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts <command>",
		Short:   "List and inspect accounts",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f), newOpenCmd(f))
	return cmd
}

func accountFields() []output.Field {
	return []output.Field{
		{Name: "CRMID", Extract: func(v any) string { return v.(api.Account).CRMID }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Account).Name }},
		{Name: "DOMAIN", Extract: func(v any) string { return v.(api.Account).Domain }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts (the API requires a --name filter)",
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
			seq := client.Accounts(cmd.Context(), api.AccountFilter{Name: name, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, accountFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by account name")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <crmId>...",
		Short: "Get one or more accounts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var out []api.Account
			for _, id := range args {
				a, err := client.GetAccount(cmd.Context(), id)
				if err != nil {
					return err
				}
				out = append(out, a)
			}
			return cmdutil.RenderSlice(f, out, accountFields())
		},
	}
}

func newOpenCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "open <crmId>",
		Short: "Open an account in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			a, err := client.GetAccount(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cmdutil.OpenResource(f.IOStreams, "account", args[0], a.CRMLink)
		},
	}
}
