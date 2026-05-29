// Package update implements `modjo update` — self-update. The actual binary
// replacement is wired to the release channel at build time; in dev builds it
// prints guidance.
package update

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmd/version"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

// NewCmdUpdate returns the update command.
func NewCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "update",
		Short:   "Update modjo to the latest version",
		GroupID: "config",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			io := f.IOStreams
			fmt.Fprintf(io.Out, "Current version: %s\n", version.String())
			if version.Version == "dev" {
				io.Errf("This is a development build; install a release via Homebrew, scoop, or the install script.\n")
				return nil
			}
			io.Errf("Self-update is handled by your package manager (brew upgrade modjo / scoop update modjo).\n")
			return nil
		},
	}
}
