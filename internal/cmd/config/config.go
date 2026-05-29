// Package config implements `modjo config`: get, set, list, edit.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	cfgpkg "github.com/tdeschamps/modjo-cli/internal/config"
)

var settableKeys = []string{"workspace", "auth_method", "base_url", "mcp_url", "output", "color", "language", "default_limit"}

// NewCmdConfig returns the config command group.
func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config <command>",
		Short:   "Manage configuration",
		GroupID: "config",
	}
	cmd.AddCommand(newGetCmd(f), newSetCmd(f), newListCmd(f), newEditCmd(f))
	return cmd
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print a configuration value for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := f.Resolver()
			if err != nil {
				return err
			}
			fmt.Fprintln(f.IOStreams.Out, r.Resolve(args[0], "", cmdutil.OSLookup))
			return nil
		},
	}
}

func newSetCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value on the active profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, val := args[0], args[1]
			if !validKey(key) {
				return cmdutil.NewUsageError(fmt.Errorf("unknown config key %q (valid: %v)", key, settableKeys))
			}
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			prof, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			if err := setField(cfg.ProfileOrDefault(prof), key, val); err != nil {
				return cmdutil.NewUsageError(err)
			}
			if err := f.SaveConfig(cfg); err != nil {
				return err
			}
			f.IOStreams.Errf("%s %s = %s (profile %q)\n", f.IOStreams.Green("✓"), key, val, prof)
			return nil
		},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List effective configuration for the active profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := f.Resolver()
			if err != nil {
				return err
			}
			keys := append([]string{}, settableKeys...)
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(f.IOStreams.Out, "%-14s %s\n", k, r.Resolve(k, "", cmdutil.OSLookup))
			}
			return nil
		},
	}
}

func newEditCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the config file in $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := f.ConfigPath
			if path == "" {
				p, err := cfgpkg.DefaultPath()
				if err != nil {
					return err
				}
				path = p
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, path)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}
}

func validKey(k string) bool {
	for _, v := range settableKeys {
		if v == k {
			return true
		}
	}
	return false
}

func setField(p *cfgpkg.Profile, key, val string) error {
	switch key {
	case "workspace":
		p.Workspace = val
	case "auth_method":
		p.AuthMethod = val
	case "base_url":
		p.BaseURL = val
	case "mcp_url":
		p.MCPURL = val
	case "output":
		p.Output = val
	case "color":
		p.Color = val
	case "language":
		p.Language = val
	case "default_limit":
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
			return fmt.Errorf("default_limit must be an integer")
		}
		p.DefaultLimit = n
	}
	return nil
}
