// Package contacts implements `modjo contacts`: list, get.
package contacts

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdContacts returns the contacts command group.
func NewCmdContacts(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contacts <command>",
		Short:   "List and inspect contacts",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f))
	return cmd
}

func contactFields() []output.Field {
	return []output.Field{
		{Name: "CRMPERSONID", Extract: func(v any) string { return v.(api.Contact).CRMPersonID }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Contact).Name }},
		{Name: "EMAIL", Extract: func(v any) string { return v.(api.Contact).Email }},
		{Name: "TITLE", Extract: func(v any) string { return v.(api.Contact).Title }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var name, account string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts",
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
			seq := client.Contacts(cmd.Context(), api.ContactFilter{Name: name, Account: account, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, contactFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by contact name")
	cmd.Flags().StringVar(&account, "account", "", "Filter by account crmId")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <crmPersonId>...",
		Short: "Get one or more contacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var out []api.Contact
			for _, id := range args {
				c, err := client.GetContact(cmd.Context(), id)
				if err != nil {
					return err
				}
				out = append(out, c)
			}
			return cmdutil.RenderSlice(f, out, contactFields())
		},
	}
}
