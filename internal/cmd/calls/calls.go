// Package calls implements `modjo calls`: list, get, transcript, summary,
// export, and open.
package calls

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdCalls returns the calls command group.
func NewCmdCalls(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "calls <command>",
		Short:   "List and inspect calls",
		GroupID: "core",
	}
	cmd.AddCommand(
		newListCmd(f),
		newGetCmd(f),
		newTranscriptCmd(f),
		newSummaryCmd(f),
		newExportCmd(f),
		newOpenCmd(f),
	)
	return cmd
}

func callFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Call).ID.String() }},
		{Name: "TITLE", Extract: func(v any) string { return v.(api.Call).Title }},
		{Name: "DATE", Extract: func(v any) string { return v.(api.Call).StartTime }},
		{Name: "SUMMARY", Extract: func(v any) string { return truncate(v.(api.Call).Summary, 60) }},
	}
}

func buildFilter(f *cmdutil.Factory, fl *listFlags) (api.CallFilter, error) {
	since, err := cmdutil.NormalizeDateFlag(f, fl.since)
	if err != nil {
		return api.CallFilter{}, err
	}
	until, err := cmdutil.NormalizeDateFlag(f, fl.until)
	if err != nil {
		return api.CallFilter{}, err
	}
	limit, err := f.EffectiveLimit()
	if err != nil {
		return api.CallFilter{}, err
	}
	return api.CallFilter{
		Account:   fl.account,
		Deal:      fl.deal,
		Contact:   fl.contact,
		User:      fl.user,
		Since:     since,
		Until:     until,
		Relations: splitCSV(fl.relations),
		Limit:     limit,
	}, nil
}

type listFlags struct {
	account, deal, contact, user, since, until, relations string
}

func bindListFlags(cmd *cobra.Command, fl *listFlags) {
	cmd.Flags().StringVar(&fl.account, "account", "", "Filter by account crmId")
	cmd.Flags().StringVar(&fl.deal, "deal", "", "Filter by deal crmId")
	cmd.Flags().StringVar(&fl.contact, "contact", "", "Filter by contact crmId")
	cmd.Flags().StringVar(&fl.user, "user", "", "Filter by user id")
	cmd.Flags().StringVar(&fl.since, "since", "", "Start date (YYYY-MM-DD or relative like 30d)")
	cmd.Flags().StringVar(&fl.until, "until", "", "End date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&fl.relations, "relations", "", "Comma-separated relations to include")
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	fl := &listFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List calls",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			filter, err := buildFilter(f, fl)
			if err != nil {
				return err
			}
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Calls(cmd.Context(), filter), callFields())
		},
	}
	bindListFlags(cmd, fl)
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	var relations string
	cmd := &cobra.Command{
		Use:   "get <callId>...",
		Short: "Get one or more calls by ID",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			rels := splitCSV(relations)
			var calls []api.Call
			for _, id := range args {
				c, err := client.GetCall(cmd.Context(), id, rels...)
				if err != nil {
					return err
				}
				calls = append(calls, c)
			}
			return cmdutil.RenderSlice(f, calls, callFields())
		},
	}
	cmd.Flags().StringVar(&relations, "relations", "", "Comma-separated relations (transcript,summary,deal,account)")
	return cmd
}

func newTranscriptCmd(f *cmdutil.Factory) *cobra.Command {
	var speakers, timestamps bool
	cmd := &cobra.Command{
		Use:   "transcript <callId>",
		Short: "Print a call transcript",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			call, err := client.GetCall(cmd.Context(), args[0], "transcript")
			if err != nil {
				return err
			}
			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			// Any machine format (json/csv/tsv/yaml) renders the raw blocks via
			// the shared Printer; only the interactive table gets the human layout.
			if !format.IsInteractive() {
				return cmdutil.RenderSlice(f, call.Transcript, transcriptFields())
			}
			io := f.IOStreams
			for _, b := range call.Transcript {
				var prefix string
				if timestamps {
					prefix += fmt.Sprintf("[%s] ", fmtTime(b.StartTime))
				}
				if speakers || !timestamps {
					prefix += io.Bold(b.SpeakerName) + ": "
				}
				fmt.Fprintf(io.Out, "%s%s\n", prefix, b.Content)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&speakers, "speakers", false, "Show speaker labels")
	cmd.Flags().BoolVar(&timestamps, "timestamps", false, "Show timestamps")
	return cmd
}

func newSummaryCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "summary <callId>",
		Short: "Print a call's pre-generated summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			call, err := client.GetCall(cmd.Context(), args[0], "summary")
			if err != nil {
				return err
			}
			if call.Summary == "" {
				f.IOStreams.Errf("No summary available for call %s\n", args[0])
				return nil
			}
			fmt.Fprintln(f.IOStreams.Out, call.Summary)
			return nil
		},
	}
}

func newExportCmd(f *cmdutil.Factory) *cobra.Command {
	fl := &listFlags{}
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export calls (defaults to CSV; use with --all)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			filter, err := buildFilter(f, fl)
			if err != nil {
				return err
			}
			// Default export format to CSV unless the user overrode it.
			if f.Flags.Output == "" && !f.Flags.JSON {
				f.Flags.Output = "csv"
			}
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Calls(cmd.Context(), filter), callFields())
		},
	}
	bindListFlags(cmd, fl)
	return cmd
}

func newOpenCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "open <callId>",
		Short: "Open a call in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			call, err := client.GetCall(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cmdutil.OpenResource(f.IOStreams, "call", args[0], call.CRMLink)
		},
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// transcriptFields describes the columns for machine-format transcript output.
func transcriptFields() []output.Field {
	return []output.Field{
		{Name: "START", Extract: func(v any) string { return fmtTime(v.(api.TranscriptBlock).StartTime) }},
		{Name: "END", Extract: func(v any) string { return fmtTime(v.(api.TranscriptBlock).EndTime) }},
		{Name: "SPEAKER", Extract: func(v any) string { return v.(api.TranscriptBlock).SpeakerName }},
		{Name: "CONTENT", Extract: func(v any) string { return v.(api.TranscriptBlock).Content }},
	}
}

// fmtTime renders a second offset as mm:ss, or hh:mm:ss for calls past an hour.
func fmtTime(sec float64) string {
	total := int(sec)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
