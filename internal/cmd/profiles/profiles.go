// Package profiles implements `modjo profiles`: list, use.
package profiles

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
)

// NewCmdProfiles returns the profiles command group.
func NewCmdProfiles(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profiles <command>",
		Short:   "List and switch profiles",
		GroupID: "config",
	}
	cmd.AddCommand(newListCmd(f), newUseCmd(f))
	return cmd
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Profiles))
			for n := range cfg.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			io := f.IOStreams
			for _, n := range names {
				marker := "  "
				if n == cfg.ActiveProfile {
					marker = io.Green("* ")
				}
				ws := cfg.Profiles[n].Workspace
				if ws != "" {
					fmt.Fprintf(io.Out, "%s%s (%s)\n", marker, n, ws)
				} else {
					fmt.Fprintf(io.Out, "%s%s\n", marker, n)
				}
			}
			return nil
		},
	}
}

func newUseCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			cfg.ProfileOrDefault(args[0])
			cfg.ActiveProfile = args[0]
			if err := f.SaveConfig(cfg); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Active profile is now %q\n", f.IOStreams.Green("✓"), args[0])
			return nil
		},
	}
}
