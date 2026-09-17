// Package callreviews implements `modjo call-reviews`: list. Call reviews are
// workspace-scoped coaching scorecards read through the REST v2 endpoint
// GET /call-reviews — they are not addressed under a single call, so this is a
// top-level group rather than a `calls` sub-command.
package callreviews

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdCallReviews returns the call-reviews command group.
func NewCmdCallReviews(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "call-reviews <command>",
		Short:   "List call reviews and their scores",
		GroupID: "core",
	}
	cmd.AddCommand(newListCmd(f))
	return cmd
}

// reviewFields describes the table columns. The per-question `answers` are
// deliberately left out: they are a nested array that no table can render
// honestly, and `--json` carries them in full.
func reviewFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.CallReview).ID.String() }},
		{Name: "CALL", Extract: func(v any) string { return v.(api.CallReview).CallID.String() }},
		{Name: "TEMPLATE", Extract: func(v any) string { return v.(api.CallReview).Template.Title }},
		{Name: "REVIEWEE", Extract: func(v any) string { return v.(api.CallReview).RevieweeID.String() }},
		{Name: "RATING", Extract: func(v any) string { return v.(api.CallReview).Rating.String() }},
		{Name: "AI", Extract: func(v any) string { return fmt.Sprint(v.(api.CallReview).IsAIGenerated) }},
		{Name: "CREATED", Extract: func(v any) string { return v.(api.CallReview).CreatedOn }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var since, until, order string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List call reviews",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if order != "" && order != "asc" && order != "desc" {
				return cmdutil.NewUsageError(fmt.Errorf("--order must be asc or desc"))
			}
			from, err := cmdutil.NormalizeDateFlag(f, since)
			if err != nil {
				return err
			}
			to, err := cmdutil.NormalizeDateFlag(f, until)
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			seq := client.CallReviews(cmd.Context(), api.CallReviewFilter{
				Since: from,
				Until: to,
				Order: order,
				Limit: limit,
			})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, reviewFields(), "call reviews")
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only reviews created on or after this date (YYYY-MM-DD or relative like 30d)")
	cmd.Flags().StringVar(&until, "until", "", "Only reviews created on or before this date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&order, "order", "", "Sort on creation date: asc|desc (default desc)")
	return cmd
}
