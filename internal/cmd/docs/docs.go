// Package docs implements `modjo docs` — open the documentation in a browser.
package docs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

const docsBaseURL = "https://cli.modjo.ai/docs"

// NewCmdDocs returns the docs command.
func NewCmdDocs(f *cmdutil.Factory) *cobra.Command {
	var web bool
	cmd := &cobra.Command{
		Use:     "docs [topic]",
		Short:   "Open the Modjo CLI documentation",
		GroupID: "config",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := docsBaseURL
			if len(args) == 1 {
				url = docsBaseURL + "/" + args[0]
			}
			if web {
				f.IOStreams.Errf("Opening %s\n", url)
				return cmdutil.OpenBrowser(url)
			}
			fmt.Fprintln(f.IOStreams.Out, url)
			return nil
		},
	}
	cmd.Flags().BoolVar(&web, "web", false, "Open in a browser instead of printing the URL")
	return cmd
}
