// Package completion implements `modjo completion` for bash/zsh/fish/powershell.
package completion

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

// NewCmdCompletion returns the completion command.
func NewCmdCompletion(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion <bash|zsh|fish|powershell>",
		Short:     "Generate a shell completion script",
		GroupID:   "config",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out := f.IOStreams.Out
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			default:
				return cmdutil.NewUsageError(fmt.Errorf("unsupported shell %q", args[0]))
			}
		},
	}
	return cmd
}
