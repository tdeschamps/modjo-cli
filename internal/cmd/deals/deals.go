// Package deals implements `modjo deals`: list and summary.
package deals

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdDeals returns the deals command group.
func NewCmdDeals(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deals <command>",
		Short:   "List and inspect deals",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f), newSummaryCmd(f))
	return cmd
}

func dealFields(io *iostreams.IOStreams) []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Deal).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { d := v.(api.Deal); return io.Hyperlink(d.Name, d.CRMLink) }},
		{Name: "STATUS", Extract: func(v any) string { return io.StatusColor(v.(api.Deal).Status) }},
		{Name: "AMOUNT", Extract: func(v any) string { return fmtAmount(v.(api.Deal)) }},
		{Name: "STAGE", Extract: func(v any) string { return v.(api.Deal).Stage }},
		{Name: "CLOSE", Extract: func(v any) string { return v.(api.Deal).CloseDate }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var name, account, status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deals",
		Long: `List deals. --status accepts a single canonical value or the aliases
open|won|lost|closed (mapped to "Open"|"Closed won"|"Closed lost"|"Closed").`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			filter := api.DealFilter{
				Name:    name,
				Account: account,
				Status:  status,
				Limit:   limit,
			}
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Deals(cmd.Context(), filter), dealFields(f.IOStreams), "deals")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by name")
	cmd.Flags().StringVar(&account, "account", "", "Filter by account ID (numeric)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (open|won|lost|closed or canonical)")
	return cmd
}

func newSummaryCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "summary <id>",
		Short: "Print a deal's AI-generated summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			summary, err := client.GetDealSummary(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			// Machine formats render the structured summary blocks; the interactive
			// table prints each block in a readable layout.
			if !format.IsInteractive() {
				return cmdutil.RenderSlice(f, summary.Data, dealSummaryFields())
			}
			if len(summary.Data) == 0 {
				f.IOStreams.Errf("No summary available for deal %s\n", args[0])
				return nil
			}
			io := f.IOStreams
			for i, b := range summary.Data {
				if i > 0 {
					fmt.Fprintln(io.Out)
				}
				if b.Type != "" {
					fmt.Fprintln(io.Out, io.Bold(b.Type))
				}
				fmt.Fprintln(io.Out, b.Value)
			}
			return nil
		},
	}
}

// dealSummaryFields describes the columns for machine-format summary output.
func dealSummaryFields() []output.Field {
	return []output.Field{
		{Name: "TYPE", Extract: func(v any) string { return v.(api.DealSummaryBlock).Type }},
		{Name: "VALUE", Extract: func(v any) string { return v.(api.DealSummaryBlock).Value }},
	}
}

func fmtAmount(d api.Deal) string {
	if d.Amount == 0 {
		return ""
	}
	s := strconv.FormatFloat(d.Amount, 'f', -1, 64)
	if d.Currency != "" {
		return d.Currency + " " + s
	}
	return s
}
