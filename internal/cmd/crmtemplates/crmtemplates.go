// Package crmtemplates implements `modjo crm-templates`: list, get, and fields.
// CRM filling templates are keyed by UUID and read through the REST v2
// management endpoints (the MCP is read-only).
package crmtemplates

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdCrmTemplates returns the crm-templates command group.
func NewCmdCrmTemplates(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "crm-templates <command>",
		Short:   "List and inspect CRM filling templates",
		GroupID: "mgmt",
	}
	cmd.AddCommand(newListCmd(f), newGetCmd(f), newFieldsCmd(f))
	return cmd
}

func templateFields() []output.Field {
	return []output.Field{
		{Name: "UUID", Extract: func(v any) string { return v.(api.CrmFillingTemplate).UUID }},
		{Name: "TITLE", Extract: func(v any) string { return v.(api.CrmFillingTemplate).Title }},
		{Name: "STATUS", Extract: func(v any) string { return v.(api.CrmFillingTemplate).Status }},
		{Name: "LANGUAGE", Extract: func(v any) string { return v.(api.CrmFillingTemplate).Language }},
	}
}

func fieldFields() []output.Field {
	return []output.Field{
		{Name: "UUID", Extract: func(v any) string { return v.(api.CrmFillingField).UUID }},
		{Name: "ORDER", Extract: func(v any) string { return v.(api.CrmFillingField).Order.String() }},
		{Name: "CRM", Extract: func(v any) string { return v.(api.CrmFillingField).CRM }},
		{Name: "ENTITY", Extract: func(v any) string { return v.(api.CrmFillingField).EntityType }},
		{Name: "KEY", Extract: func(v any) string { return v.(api.CrmFillingField).FieldKey }},
		{Name: "TYPE", Extract: func(v any) string { return v.(api.CrmFillingField).FieldType }},
		{Name: "PROMPT", Extract: func(v any) string { return v.(api.CrmFillingField).Prompt }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CRM filling templates",
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
			seq := client.CrmFillingTemplates(cmd.Context(), api.CrmFillingTemplateFilter{Status: status, Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, templateFields(), "crm templates")
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: pending|published")
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <uuid>...",
		Short: "Get one or more CRM filling templates",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			return cmdutil.GetAndRender(cmd.Context(), f, args, client.GetCrmFillingTemplate, templateFields())
		},
	}
}

func newFieldsCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "fields <uuid>",
		Short: "List the fields of a CRM filling template",
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
			seq := client.CrmFillingTemplateFields(cmd.Context(), args[0], api.PageFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, fieldFields(), "fields")
		},
	}
}
