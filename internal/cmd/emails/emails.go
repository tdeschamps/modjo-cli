// Package emails implements `modjo emails`: list, get.
package emails

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdEmails returns the emails command group.
func NewCmdEmails(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "emails <command>",
		Short:   "List and inspect emails",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f))
	return cmd
}

func emailFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Email).ID.String() }},
		{Name: "SUBJECT", Extract: func(v any) string { return v.(api.Email).Subject }},
		{Name: "FROM", Extract: func(v any) string { return v.(api.Email).From }},
		{Name: "DATE", Extract: func(v any) string { return v.(api.Email).Date }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var account, deal, since, until string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List emails",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			s, err := cmdutil.NormalizeDateFlag(f, since)
			if err != nil {
				return err
			}
			u, err := cmdutil.NormalizeDateFlag(f, until)
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			seq := client.Emails(cmd.Context(), api.EmailFilter{Account: account, Deal: deal, Since: s, Until: u, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, emailFields())
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Filter by account crmId")
	cmd.Flags().StringVar(&deal, "deal", "", "Filter by deal crmId")
	cmd.Flags().StringVar(&since, "since", "", "Start date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&until, "until", "", "End date (YYYY-MM-DD or relative)")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <emailId>",
		Short: "Get one email (includes content)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			e, err := client.GetEmail(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			if format == output.FormatTable {
				io := f.IOStreams
				fmt.Fprintf(io.Out, "%s %s\n", io.Bold("Subject:"), e.Subject)
				fmt.Fprintf(io.Out, "%s %s\n", io.Bold("From:"), e.From)
				fmt.Fprintf(io.Out, "%s %s\n\n", io.Bold("Date:"), e.Date)
				fmt.Fprintln(io.Out, e.Content)
				return nil
			}
			return cmdutil.RenderSlice(f, []api.Email{e}, emailFields())
		},
	}
}
