// Package ask implements `modjo ask` — the natural-language layer over the MCP
// ask_anything_on_* tools. It resolves friendly agent names to UUIDs, applies
// the configured language, and enforces the upstream 60s analysis timeout
// (surfaced as exit code 124).
package ask

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/mcp"
)

// analysisTimeout matches the upstream AI-analysis limit (product spec §7.7).
const analysisTimeout = 60 * time.Second

type askType int

const (
	askCall askType = iota
	askDeal
	askAccount
)

// NewCmdAsk returns the ask command group.
func NewCmdAsk(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ask <call|deal|account> <id> <question>",
		Short:   "Ask a natural-language question about a call, deal, or account",
		GroupID: "ai",
		Long: `Ask runs a natural-language query against Modjo's AI over a specific
entity. Examples:

  modjo ask call 74969 "What objections came up and how were they handled?"
  modjo ask deal 006XYZ "What are the risks and the single best next step?" --agent DealBriefing`,
	}

	cmd.AddCommand(
		newAskSub(f, "call", askCall, "<callId>"),
		newAskSub(f, "deal", askDeal, "<crmId>"),
		newAskSub(f, "account", askAccount, "<crmId>"),
	)
	return cmd
}

func newAskSub(f *cmdutil.Factory, name string, typ askType, idHint string) *cobra.Command {
	var agent, language string
	var stream bool
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s %s <question>", name, idHint),
		Short: fmt.Sprintf("Ask about a %s", name),
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			question := strings.Join(args[1:], " ")

			client, err := f.MCPClient()
			if err != nil {
				return err
			}

			// Resolve the agent name → UUID, and default language from profile.
			agentID, err := resolveAgent(cmd.Context(), f, agent)
			if err != nil {
				return err
			}
			lang := language
			if lang == "" {
				if r, err := f.Resolver(); err == nil {
					lang = r.Resolve("language", "", cmdutil.OSLookup)
				}
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), analysisTimeout)
			defer cancel()

			opt := mcp.AskOpts{Agent: agentID, Language: lang}
			var ans mcp.Answer
			switch typ {
			case askCall:
				ans, err = client.AskOnCall(ctx, id, question, opt)
			case askDeal:
				ans, err = client.AskOnDeal(ctx, id, question, opt)
			case askAccount:
				ans, err = client.AskOnAccount(ctx, id, question, opt)
			}
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return cmdutil.NewSilentError(cmdutil.ExitTimeout,
						fmt.Errorf("AI analysis timed out after %s; try narrowing scope (fewer calls, a tighter --since)", analysisTimeout))
				}
				return err
			}

			format, err := f.OutputFormat()
			if err != nil {
				return err
			}
			if !format.IsInteractive() {
				p, _ := f.Printer()
				envelope := struct {
					Answer string `json:"answer"`
					Agent  string `json:"agent,omitempty"`
					Entity string `json:"entity"`
					Type   string `json:"type"`
				}{ans.Answer, agentID, id, name}
				return p.PrintJSON(envelope)
			}
			fmt.Fprintln(f.IOStreams.Out, ans.Answer)
			_ = stream
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "Agent UUID or friendly name (e.g. DealBriefing)")
	cmd.Flags().StringVar(&language, "language", "", "Response language (defaults to profile)")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream tokens as they arrive (when supported)")
	return cmd
}

// resolveAgent turns a UUID or friendly name into a UUID. Native agent names
// resolve offline; unknown names are looked up via `agents list`.
func resolveAgent(ctx context.Context, f *cmdutil.Factory, agent string) (string, error) {
	if agent == "" {
		return "", nil
	}
	if looksLikeUUID(agent) {
		return agent, nil
	}
	if uuid, ok := mcp.NativeAgents[agent]; ok {
		return uuid, nil
	}
	// Fall back to a server lookup by name (case-insensitive).
	client, err := f.APIClient()
	if err != nil {
		return "", err
	}
	for a, err := range client.Agents(ctx, api.AgentFilter{Search: agent}) {
		if err != nil {
			return "", err
		}
		if strings.EqualFold(a.Name, agent) {
			return a.UUID, nil
		}
	}
	return "", fmt.Errorf("could not resolve agent %q to a UUID (try `modjo agents list`)", agent)
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}
