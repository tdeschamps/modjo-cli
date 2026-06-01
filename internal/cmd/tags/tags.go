// Package tags implements `modjo tags`: list.
package tags

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdTags returns the tags command group.
func NewCmdTags(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tags <command>",
		Short:   "List call tags",
		GroupID: "mgmt",
	}
	cmd.AddCommand(newListCmd(f))
	return cmd
}

func tagFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Tag).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Tag).Name }},
		{Name: "COLOR", Extract: func(v any) string { return v.(api.Tag).Color }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tags",
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
			seq := client.Tags(cmd.Context(), api.TagFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, tagFields(), "tags")
		},
	}
}
