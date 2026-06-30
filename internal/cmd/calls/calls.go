// Package calls implements `modjo calls`: list, get, transcript, summary,
// export, upload, notes, next-steps, crm-answers, and a tags sub-group
// (list/add/remove).
package calls

import (
	"context"
	"fmt"
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
		newUploadCmd(f),
		newNotesCmd(f),
		newNextStepsCmd(f),
		newCrmAnswersCmd(f),
		newTagsCmd(f),
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
			blocks, err := client.GetCallTranscript(cmd.Context(), args[0])
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
			summaries, err := client.GetCallSummaries(cmd.Context(), args[0])
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

func newNotesCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "notes <callId>",
		Short: "List a call's notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			notes, err := client.GetCallNotes(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cmdutil.RenderSlice(f, notes, noteFields())
		},
	}
}

func newNextStepsCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "next-steps <callId>",
		Short: "List a call's AI-extracted next steps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			steps, err := client.GetCallNextSteps(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cmdutil.RenderSlice(f, steps, nextStepFields())
		},
	}
}

func newCrmAnswersCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "crm-answers <callId>",
		Short: "List CRM filling answers pushed for a call",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			limit, err := f.EffectiveLimit()
			if err != nil {
				return err
			}
			seq := client.CrmFillingAnswers(cmd.Context(), args[0], api.PageFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, crmAnswerFields(), "answers")
		},
	}
}

func newTagsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags <command>",
		Short: "List and manage a call's tags",
	}
	cmd.AddCommand(
		newTagsListCmd(f),
		newTagsAddCmd(f),
		newTagsRemoveCmd(f),
	)
	return cmd
}

func newTagsListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list <callId>",
		Short: "List a call's tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			tags, err := client.GetCallTags(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cmdutil.RenderSlice(f, tags, tagFields())
		},
	}
}

func newTagsAddCmd(f *cmdutil.Factory) *cobra.Command {
	var tag int
	cmd := &cobra.Command{
		Use:   "add <callId> --tag <tagId>",
		Short: "Add a tag to a call",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("tag") {
				return cmdutil.NewUsageError(fmt.Errorf("--tag is required"))
			}
			id := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would add tag %d to call %s\n", tag, id)
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			ct, err := client.AddCallTag(cmd.Context(), id, tag)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Added tag %s to call %s\n", f.IOStreams.Green("✓"), ct.TagID.String(), ct.CallID.String())
			return cmdutil.RenderSlice(f, []api.CallTag{ct}, callTagFields())
		},
	}
	cmd.Flags().IntVar(&tag, "tag", 0, "ID of the tag to add (required)")
	return cmd
}

func newTagsRemoveCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <callId> <tagId>",
		Short: "Remove a tag from a call",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, tagID := args[0], args[1]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would remove tag %s from call %s\n", tagID, id)
				return nil
			}
			if !f.Flags.Yes {
				ok, err := cmdutil.Confirm(f.IOStreams, fmt.Sprintf("Remove tag %s from call %s?", tagID, id), false)
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
			if err := client.RemoveCallTag(cmd.Context(), id, tagID); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Removed tag %s from call %s\n", f.IOStreams.Green("✓"), tagID, id)
			return nil
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
		{Name: "ANSWER", Extract: func(v any) string { return v.(api.CallSummary).Answer }},
	}
}

// noteFields describes the columns for call note output.
func noteFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Note).ID.String() }},
		{Name: "TITLE", Extract: func(v any) string { return v.(api.Note).Title }},
		{Name: "STATUS", Extract: func(v any) string { return v.(api.Note).Status }},
		{Name: "TYPE", Extract: func(v any) string { return v.(api.Note).Type }},
		{Name: "DATE", Extract: func(v any) string { return v.(api.Note).Date }},
	}
}

// nextStepFields describes the columns for call next-step output.
func nextStepFields() []output.Field {
	return []output.Field{
		{Name: "TITLE", Extract: func(v any) string { return v.(api.NextStepItem).Title }},
		{Name: "DESCRIPTION", Extract: func(v any) string { return v.(api.NextStepItem).Description }},
	}
}

// tagFields describes the columns for tag output (reused by `tags list`).
func tagFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Tag).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Tag).Name }},
		{Name: "COLOR", Extract: func(v any) string { return v.(api.Tag).Color }},
	}
}

// callTagFields describes the columns for a call/tag association.
func callTagFields() []output.Field {
	return []output.Field{
		{Name: "CALLID", Extract: func(v any) string { return v.(api.CallTag).CallID.String() }},
		{Name: "TAGID", Extract: func(v any) string { return v.(api.CallTag).TagID.String() }},
	}
}

// crmAnswerFields describes the columns for CRM filling answer output.
func crmAnswerFields() []output.Field {
	return []output.Field{
		{Name: "UUID", Extract: func(v any) string { return v.(api.CrmFillingAnswer).UUID }},
		{Name: "FIELD", Extract: func(v any) string { return v.(api.CrmFillingAnswer).CrmFillingFieldUUID }},
		{Name: "CRMID", Extract: func(v any) string { return v.(api.CrmFillingAnswer).CRMID }},
		{Name: "MODIFIED", Extract: func(v any) string { return v.(api.CrmFillingAnswer).ModifiedOn }},
	}
}

// newUploadCmd uploads a call by recording URL (POST /calls -> 202). Modjo
// downloads and processes the recording asynchronously.
func newUploadCmd(f *cmdutil.Factory) *cobra.Command {
	var (
		mediaURL, date, name, direction, account, deal string
		duration                                       float64
		participants                                   []string
		tags                                           []string
	)
	cmd := &cobra.Command{
		Use:   "upload --media-url <url> --date <date> --participant <email:type[:name]>...",
		Short: "Upload a call by recording URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if mediaURL == "" || date == "" || len(participants) == 0 {
				return cmdutil.NewUsageError(fmt.Errorf("--media-url, --date and at least one --participant are required"))
			}
			if direction != "" && direction != "inbound" && direction != "outbound" {
				return cmdutil.NewUsageError(fmt.Errorf("--direction must be inbound or outbound"))
			}
			parts, err := parseParticipants(participants)
			if err != nil {
				return cmdutil.NewUsageError(err)
			}
			acct, err := parseCRMRef(account)
			if err != nil {
				return cmdutil.NewUsageError(fmt.Errorf("--account: %w", err))
			}
			dl, err := parseCRMRef(deal)
			if err != nil {
				return cmdutil.NewUsageError(fmt.Errorf("--deal: %w", err))
			}
			in := api.UploadCallInput{
				DownloadMediaURL: mediaURL,
				Name:             name,
				Date:             date,
				Direction:        direction,
				Duration:         duration,
				Participants:     parts,
				Tags:             tags,
				Account:          acct,
				Deal:             dl,
			}
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would upload call %q (%d participant(s))\n", mediaURL, len(parts))
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			resp, err := client.UploadCall(cmd.Context(), in)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Uploaded call %s (status %s)\n", f.IOStreams.Green("✓"), resp.CallID, resp.Status)
			return cmdutil.RenderSlice(f, []api.UploadCallResponse{resp}, uploadFields())
		},
	}
	cmd.Flags().StringVar(&mediaURL, "media-url", "", "Recording URL Modjo downloads (required)")
	cmd.Flags().StringVar(&date, "date", "", "Call date/time, ISO-8601 (required)")
	cmd.Flags().StringVar(&name, "name", "", "Call title")
	cmd.Flags().StringVar(&direction, "direction", "", "Call direction: inbound|outbound")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Call duration in seconds")
	cmd.Flags().StringArrayVar(&participants, "participant", nil, "Participant as email:type[:name] (type=user|contact; repeatable; required)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to attach (repeatable)")
	cmd.Flags().StringVar(&account, "account", "", "Attach an account as crm:crmId")
	cmd.Flags().StringVar(&deal, "deal", "", "Attach a deal as crm:crmId")
	return cmd
}

// uploadFields describes the columns for an upload response.
func uploadFields() []output.Field {
	return []output.Field{
		{Name: "CALL_ID", Extract: func(v any) string { return v.(api.UploadCallResponse).CallID }},
		{Name: "STATUS", Extract: func(v any) string { return v.(api.UploadCallResponse).Status }},
	}
}

// parseParticipants turns "email:type[:name]" strings into UploadCallParticipant
// values. type must be "user" or "contact".
func parseParticipants(specs []string) ([]api.UploadCallParticipant, error) {
	out := make([]api.UploadCallParticipant, 0, len(specs))
	for _, s := range specs {
		fields := strings.SplitN(s, ":", 3)
		if len(fields) < 2 || fields[0] == "" || fields[1] == "" {
			return nil, fmt.Errorf("participant %q must be email:type[:name]", s)
		}
		if fields[1] != "user" && fields[1] != "contact" {
			return nil, fmt.Errorf("participant %q: type must be user or contact", s)
		}
		p := api.UploadCallParticipant{Email: fields[0], Type: fields[1]}
		if len(fields) == 3 {
			p.Name = fields[2]
		}
		out = append(out, p)
	}
	return out, nil
}

// parseCRMRef turns "crm:crmId" into a *CRMRef, or nil for an empty spec.
func parseCRMRef(spec string) (*api.CRMRef, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	crm, crmID, ok := strings.Cut(spec, ":")
	if !ok || crm == "" || crmID == "" {
		return nil, fmt.Errorf("%q must be crm:crmId", spec)
	}
	return &api.CRMRef{CRM: crm, CRMID: crmID}, nil
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
