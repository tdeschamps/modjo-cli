// Package deals implements `modjo deals`: list, get, open.
package deals

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdDeals returns the deals command group.
func NewCmdDeals(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deals <command>",
		Short:   "List and inspect deals",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f), newOpenCmd(f))
	return cmd
}

func dealFields() []output.Field {
	return []output.Field{
		{Name: "CRMID", Extract: func(v any) string { return v.(api.Deal).CRMID }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Deal).Name }},
		{Name: "ACCOUNT", Extract: func(v any) string { return v.(api.Deal).Account }},
		{Name: "STATUS", Extract: func(v any) string { return v.(api.Deal).Status }},
		{Name: "AMOUNT", Extract: func(v any) string { return fmtAmount(v.(api.Deal)) }},
		{Name: "CLOSE", Extract: func(v any) string { return v.(api.Deal).CloseDate }},
		{Name: "SOURCE", Extract: func(v any) string { return v.(api.Deal).Source }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var status, source []string
	var account, closeBefore, closeAfter, lossReason string
	var amountMin, amountMax float64
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deals",
		Long: `List deals. --status accepts canonical values or the aliases
open|won|lost|closed (mapped to "Open"|"Closed won"|"Closed lost"|"Closed").`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			cb, err := cmdutil.NormalizeDateFlag(f, closeBefore)
			if err != nil {
				return err
			}
			ca, err := cmdutil.NormalizeDateFlag(f, closeAfter)
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			filter := api.DealFilter{
				Status:      status,
				Account:     account,
				CloseBefore: cb,
				CloseAfter:  ca,
				AmountMin:   amountMin,
				AmountMax:   amountMax,
				Source:      source,
				LossReason:  lossReason,
				Limit:       limit,
			}
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Deals(cmd.Context(), filter), dealFields())
		},
	}
	cmd.Flags().StringSliceVar(&status, "status", nil, "Filter by status (open|won|lost|closed or canonical)")
	cmd.Flags().StringVar(&account, "account", "", "Filter by account crmId")
	cmd.Flags().StringVar(&closeBefore, "close-before", "", "Close date before (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&closeAfter, "close-after", "", "Close date after (YYYY-MM-DD or relative)")
	cmd.Flags().Float64Var(&amountMin, "amount-min", 0, "Minimum amount")
	cmd.Flags().Float64Var(&amountMax, "amount-max", 0, "Maximum amount")
	cmd.Flags().StringSliceVar(&source, "source", nil, "Filter by source(s)")
	cmd.Flags().StringVar(&lossReason, "loss-reason", "", "Filter by loss reason")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <crmId>...",
		Short: "Get one or more deals",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var deals []api.Deal
			for _, id := range args {
				d, err := client.GetDeal(cmd.Context(), id)
				if err != nil {
					return err
				}
				deals = append(deals, d)
			}
			p, err := f.Printer()
			if err != nil {
				return err
			}
			items := make([]any, len(deals))
			for i, d := range deals {
				items[i] = d
			}
			return p.Output(deals, items, dealFields())
		},
	}
}

func newOpenCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "open <crmId>",
		Short: "Open a deal in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			d, err := client.GetDeal(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if d.CRMLink == "" {
				return fmt.Errorf("deal %s has no CRM link", args[0])
			}
			f.IOStreams.Errf("Opening %s\n", d.CRMLink)
			return cmdutil.OpenBrowser(d.CRMLink)
		},
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
