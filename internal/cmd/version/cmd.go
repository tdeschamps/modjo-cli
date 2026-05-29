package version

import (
	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

// NewCmdVersion returns the `modjo version` command.
func NewCmdVersion(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version, commit, and build date",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(String())
			return nil
		},
	}
}
