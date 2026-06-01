// Package topics implements `modjo topics`: list.
package topics

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdTopics returns the topics command group.
func NewCmdTopics(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "topics <command>",
		Short:   "List conversation topics",
		GroupID: "mgmt",
	}
	cmd.AddCommand(newListCmd(f))
	return cmd
}

func topicFields() []output.Field {
	return []output.Field{
		{Name: "ID", Extract: func(v any) string { return v.(api.Topic).ID.String() }},
		{Name: "NAME", Extract: func(v any) string { return v.(api.Topic).Name }},
		{Name: "SLUG", Extract: func(v any) string { return v.(api.Topic).Slug }},
		{Name: "COLOR", Extract: func(v any) string { return v.(api.Topic).Color }},
		{Name: "SAID BY", Extract: func(v any) string { return v.(api.Topic).SaidBy }},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List topics",
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
			seq := client.Topics(cmd.Context(), api.TopicFilter{Limit: limit})
			return cmdutil.CollectAndRender(cmd.Context(), f, seq, topicFields(), "topics")
		},
	}
}
