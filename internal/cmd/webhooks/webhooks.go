// Package webhooks implements `modjo webhooks`: list, get, create, update,
// delete. Webhooks are keyed by UUID (not a numeric id). Writes go through the
// REST v2 management endpoints (the MCP is read-only) and honor --dry-run and
// --yes.
package webhooks

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdWebhooks returns the webhooks command group.
func NewCmdWebhooks(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhooks <command>",
		Short:   "List and manage webhooks",
		GroupID: "mgmt",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f), newCreateCmd(f), newUpdateCmd(f), newDeleteCmd(f))
	return cmd
}

func webhookFields() []output.Field {
	return []output.Field{
		{Name: "UUID", Extract: func(v any) string { return v.(api.Webhook).UUID }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Webhook).Name }},
		{Name: "URL", Extract: func(v any) string { return v.(api.Webhook).URL }},
		{Name: "EVENTS", Extract: func(v any) string { return strings.Join(v.(api.Webhook).Events, ",") }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List webhooks",
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
			seq := client.Webhooks(cmd.Context(), api.WebhookFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, webhookFields(), "webhooks")
		},
	}
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <uuid>...",
		Short: "Get one or more webhooks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetWebhook, webhookFields())
		},
	}
}

func newCreateCmd(f *cmdutil.Factory) *cobra.Command {
	var name, url string
	var events []string
	cmd := &cobra.Command{
		Use:   "create --name <name> --url <url> --event <event>...",
		Short: "Create a webhook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || url == "" || len(events) == 0 {
				return cmdutil.NewUsageError(fmt.Errorf("--name, --url and at least one --event are required"))
			}
			in := api.CreateWebhookInput{Name: name, URL: url, Events: events}
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would create webhook %q -> %s (events=%s)\n", name, url, strings.Join(events, ","))
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			w, err := client.CreateWebhook(cmd.Context(), in)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Created webhook %s (uuid %s)\n", f.IOStreams.Green("✓"), w.Name, w.UUID)
			return cmdutil.RenderSlice(f, []api.Webhook{w}, webhookFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name of the new webhook (required)")
	cmd.Flags().StringVar(&url, "url", "", "Destination URL for the new webhook (required)")
	cmd.Flags().StringArrayVar(&events, "event", nil, "Event to subscribe to (repeatable; required): call_summarized|call_recording_deleted|call_transcript_deleted")
	return cmd
}

func newUpdateCmd(f *cmdutil.Factory) *cobra.Command {
	var name, url string
	var events []string
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Update a webhook (partial)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			// Send only the flags the user set, so --name "" clears the name
			// rather than being mistaken for "no flag given".
			var in api.UpdateWebhookInput
			if cmd.Flags().Changed("name") {
				in.Name = api.Ptr(name)
			}
			if cmd.Flags().Changed("url") {
				in.URL = api.Ptr(url)
			}
			if cmd.Flags().Changed("event") {
				in.Events = events
			}
			if in.Name == nil && in.URL == nil && in.Events == nil {
				return cmdutil.NewUsageError(fmt.Errorf("at least one of --name, --url or --event is required"))
			}
			if f.Flags.DryRun {
				var changes []string
				if in.Name != nil {
					changes = append(changes, fmt.Sprintf("name=%q", *in.Name))
				}
				if in.URL != nil {
					changes = append(changes, fmt.Sprintf("url=%s", *in.URL))
				}
				if in.Events != nil {
					changes = append(changes, fmt.Sprintf("events=%s", strings.Join(in.Events, ",")))
				}
				f.IOStreams.Errf("[dry-run] would update webhook %s (%s)\n", uuid, strings.Join(changes, " "))
				return nil
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			w, err := client.UpdateWebhook(cmd.Context(), uuid, in)
			if err != nil {
				return err
			}
			f.IOStreams.Errf("%s Updated webhook %s\n", f.IOStreams.Green("✓"), w.UUID)
			return cmdutil.RenderSlice(f, []api.Webhook{w}, webhookFields())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&url, "url", "", "New destination URL")
	cmd.Flags().StringArrayVar(&events, "event", nil, "Replacement event to subscribe to (repeatable)")
	return cmd
}

func newDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uuid := args[0]
			if f.Flags.DryRun {
				f.IOStreams.Errf("[dry-run] would delete webhook %s\n", uuid)
				return nil
			}
			if !f.Flags.Yes {
				ok, err := cmdutil.Confirm(f.IOStreams, fmt.Sprintf("Delete webhook %s?", uuid), false)
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
			if err := client.DeleteWebhook(cmd.Context(), uuid); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Deleted webhook %s\n", f.IOStreams.Green("✓"), uuid)
			return nil
		},
	}
}
