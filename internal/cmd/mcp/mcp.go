// Package mcp implements `modjo mcp`: serve, tools, call, config.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	mcpclient "github.com/tdeschamps/modjo-cli/internal/mcp"
)

// NewCmdMCP returns the mcp command group.
func NewCmdMCP(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mcp <command>",
		Short:   "Run and inspect the Modjo MCP integration",
		GroupID: "ai",
	}
	cmd.AddCommand(newServeCmd(f), newToolsCmd(f), newCallCmd(f), newConfigCmd(f))
	return cmd
}

func newServeCmd(f *cmdutil.Factory) *cobra.Command {
	var transport string
	var port int
	var tools []string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a local MCP server exposing Modjo tools",
		Long: `Run a local MCP server that re-exposes the Modjo tools, authenticating
upstream with your stored credential so MCP clients never touch the raw key.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if transport != "stdio" {
				return cmdutil.NewUsageError(fmt.Errorf("only the stdio transport is implemented (got %q)", transport))
			}
			client, err := f.MCPClient()
			if err != nil {
				return err
			}
			_ = tools
			_ = port
			srv := mcpclient.NewServer(client)
			f.IOStreams.Errf("modjo MCP server listening on stdio\n")
			return srv.ServeStdio(cmd.Context(), f.IOStreams.In, f.IOStreams.Out)
		},
	}
	cmd.Flags().StringVar(&transport, "transport", "stdio", "Transport: stdio|http")
	cmd.Flags().IntVar(&port, "port", 7000, "Port for the http transport")
	cmd.Flags().StringSliceVar(&tools, "tools", nil, "Expose only a subset of tools")
	return cmd
}

func newToolsCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "List tools the upstream Modjo MCP exposes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.MCPClient()
			if err != nil {
				return err
			}
			tools, err := client.Tools(cmd.Context())
			if err != nil {
				return err
			}
			format, _ := f.OutputFormat()
			if !format.IsInteractive() {
				p, _ := f.Printer()
				return p.PrintJSON(tools)
			}
			io := f.IOStreams
			for _, t := range tools {
				fmt.Fprintf(io.Out, "%s\t%s\n", io.Bold(t.Name), t.Description)
			}
			return nil
		},
	}
}

func newCallCmd(f *cmdutil.Factory) *cobra.Command {
	var argsJSON string
	cmd := &cobra.Command{
		Use:   "call <tool>",
		Short: "Invoke one MCP tool and print the JSON result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.MCPClient()
			if err != nil {
				return err
			}
			var toolArgs map[string]any
			if argsJSON != "" {
				if err := json.Unmarshal([]byte(argsJSON), &toolArgs); err != nil {
					return cmdutil.NewUsageError(fmt.Errorf("--args must be valid JSON: %w", err))
				}
			}
			raw, err := client.Call(cmd.Context(), args[0], toolArgs)
			if err != nil {
				return err
			}
			var pretty any
			_ = json.Unmarshal(raw, &pretty)
			p, _ := f.Printer()
			return p.PrintJSON(pretty)
		},
	}
	cmd.Flags().StringVar(&argsJSON, "args", "", `Tool arguments as JSON, e.g. '{"filters":{}}'`)
	return cmd
}

func newConfigCmd(f *cmdutil.Factory) *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print ready-to-paste MCP client config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil || exe == "" {
				exe = "modjo"
			}
			if lp, lerr := exec.LookPath("modjo"); lerr == nil {
				exe = lp
			}
			snippet := map[string]any{
				"mcpServers": map[string]any{
					"modjo": map[string]any{
						"command": exe,
						"args":    []string{"mcp", "serve"},
					},
				},
			}
			out, _ := json.MarshalIndent(snippet, "", "  ")
			io := f.IOStreams
			switch client {
			case "claude-desktop":
				io.Errf("Paste into claude_desktop_config.json:\n")
			case "cursor":
				io.Errf("Paste into your Cursor MCP settings:\n")
			case "codex":
				io.Errf("Paste into your Codex MCP settings:\n")
			}
			fmt.Fprintln(io.Out, string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client: claude-desktop|cursor|codex")
	return cmd
}
