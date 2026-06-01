// Package calls implements `modjo calls`: list, get, transcript, summary, and
// export.
package calls

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
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
	)
	return cmd
}

func callFields(io *iostreams.IOStreams) []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Call).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { return io.Bold(v.(api.Call).Title) }},
		{Name: "DATE", Extract: func(v any) string { return v.(api.Call).StartTime }},
		{Name: "DURATION", Extract: func(v any) string { return fmtTime(v.(api.Call).Duration) }},
		{Name: "DIRECTION", Extract: func(v any) string { return v.(api.Call).Direction }},
		{Name: "STATUS", Extract: func(v any) string { return v.(api.Call).Status }},
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
		Account: fl.account,
		Deal:    fl.deal,
		User:    fl.user,
		Since:   since,
		Until:   until,
		Expand:  splitCSV(fl.expand),
		Limit:   limit,
	}, nil
}

type listFlags struct {
	account, deal, user, since, until, expand string
}

func bindListFlags(cmd *cobra.Command, fl *listFlags) {
	cmd.Flags().StringVar(&fl.account, "account", "", "Filter by account id (numeric)")
	cmd.Flags().StringVar(&fl.deal, "deal", "", "Filter by deal id (numeric)")
	cmd.Flags().StringVar(&fl.user, "user", "", "Filter by user id (numeric)")
	cmd.Flags().StringVar(&fl.since, "since", "", "Start date (YYYY-MM-DD or relative like 30d)")
	cmd.Flags().StringVar(&fl.until, "until", "", "End date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&fl.expand, "expand", "", "Comma-separated relations to expand (contacts,deal,account,users)")
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
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Calls(cmd.Context(), filter), callFields(f.IOStreams), "calls")
		},
	}
	bindListFlags(cmd, fl)
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	var expand string
	cmd := &cobra.Command{
		Use:   "get <callId>...",
		Short: "Get one or more calls by ID",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			rels := splitCSV(expand)
			get := func(ctx context.Context, id string) (api.Call, error) {
				return client.GetCall(ctx, id, rels...)
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, get, callFields(f.IOStreams))
		},
	}
	cmd.Flags().StringVar(&expand, "expand", "", "Comma-separated relations to expand (contacts,deal,account,users)")
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
			blocks, err := fetchTranscript(cmd.Context(), client, args[0])
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
				return cmdutil.RenderSlice(f, blocks, transcriptFields())
			}
			io := f.IOStreams
			for _, b := range blocks {
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
		Short: "Print a call's pre-generated summaries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			summaries, err := fetchSummaries(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			// Machine formats render the structured summary rows; the interactive
			// table prints each summary's answer in a readable layout.
			if !format.IsInteractive() {
				return cmdutil.RenderSlice(f, summaries, summaryFields())
			}
			if len(summaries) == 0 {
				f.IOStreams.Errf("No summary available for call %s\n", args[0])
				return nil
			}
			io := f.IOStreams
			for i, s := range summaries {
				if i > 0 {
					fmt.Fprintln(io.Out)
				}
				if title := strings.TrimSpace(s.TemplateTitle); title != "" {
					fmt.Fprintln(io.Out, io.Bold(title))
				}
				fmt.Fprintln(io.Out, s.Answer)
			}
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
			return cmdutil.CollectAndRender(cmd.Context(), f, client.Calls(cmd.Context(), filter), callFields(f.IOStreams), "calls")
		},
	}
	bindListFlags(cmd, fl)
	return cmd
}

// dataEnvelope is the {data:[...]} wrapper the call sub-resource endpoints
// (transcript, summaries) return. We decode it locally because the Foundation
// client exposes these only via Raw.
type dataEnvelope[T any] struct {
	Data []T `json:"data"`
}

// fetchTranscript reads GET /calls/{id}/transcript and returns its blocks. The
// endpoint yields an empty list while the call is still processing.
func fetchTranscript(ctx context.Context, client *api.Client, id string) ([]api.TranscriptBlock, error) {
	raw, err := client.Raw(ctx, http.MethodGet, "/calls/"+url.PathEscape(id)+"/transcript", nil, nil)
	if err != nil {
		return nil, err
	}
	var env dataEnvelope[api.TranscriptBlock]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// fetchSummaries reads GET /calls/{id}/summaries.
func fetchSummaries(ctx context.Context, client *api.Client, id string) ([]api.CallSummary, error) {
	raw, err := client.Raw(ctx, http.MethodGet, "/calls/"+url.PathEscape(id)+"/summaries", nil, nil)
	if err != nil {
		return nil, err
	}
	var env dataEnvelope[api.CallSummary]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
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

// summaryFields describes the columns for machine-format summary output.
func summaryFields() []output.Field {
	return []output.Field{
		{Name: "TEMPLATE", Extract: func(v any) string { return v.(api.CallSummary).TemplateTitle }},
		{Name: "LENGTH", Extract: func(v any) string { return v.(api.CallSummary).TemplateLength }},
		{Name: "ANSWER", Extract: func(v any) string { return truncate(v.(api.CallSummary).Answer, 80) }},
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
