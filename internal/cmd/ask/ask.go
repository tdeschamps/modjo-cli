// Package ask implements `modjo ask` — the natural-language layer over the MCP
// ask_anything_on_* tools. It applies the configured language and enforces the
// upstream 60s analysis timeout (surfaced as exit code 124).
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

// NewCmdAsk returns the ask command group. With no subcommand on an interactive
// terminal it drops into a guided picker (choose entity type → resolve → ask).
func NewCmdAsk(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ask <call|deal|account> <id> <question>",
		Short:   "Ask a natural-language question about a call, deal, or account",
		GroupID: "ai",
		Long: `Ask runs a natural-language query against Modjo's AI over a specific
entity. Examples:

  modjo ask call 74969 "What objections came up and how were they handled?"
  modjo ask deal 006XYZ "What are the risks and the single best next step?"

Run 'modjo ask' with no arguments on a terminal for a guided prompt.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !f.IOStreams.CanPrompt() {
				return cmd.Help()
			}
			return runInteractive(cmd, f)
		},
	}

	cmd.AddCommand(
		newAskSub(f, "call", askCall, "<callId>"),
		newAskSub(f, "deal", askDeal, "<crmId>"),
		newAskSub(f, "account", askAccount, "<crmId>"),
	)
	return cmd
}

// askParams carries everything runAsk needs.
type askParams struct {
	typ      askType
	name     string
	id       string
	question string
	language string
}

// runAsk runs the MCP analysis behind a spinner (enforcing the 60s timeout →
// exit 124) and renders the answer (plain on a TTY, a JSON envelope when piped).
// Shared by the typed subcommands and the interactive flow.
func runAsk(cmd *cobra.Command, f *cmdutil.Factory, p askParams) error {
	client, err := f.MCPClient()
	if err != nil {
		return err
	}
	lang := p.language
	if lang == "" {
		if r, rerr := f.Resolver(); rerr == nil {
			lang = r.Resolve("language", "", cmdutil.OSLookup)
		}
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), analysisTimeout)
	defer cancel()

	// AI analysis can take up to a minute; show a spinner while we wait
	// (no-op when piped/--quiet so the answer on stdout stays clean).
	sp := f.IOStreams.NewSpinner(fmt.Sprintf("Analyzing %s %s…", p.name, p.id))
	sp.Start()

	opt := mcp.AskOpts{Language: lang}
	var ans mcp.Answer
	switch p.typ {
	case askCall:
		ans, err = client.AskOnCall(ctx, p.id, p.question, opt)
	case askDeal:
		ans, err = client.AskOnDeal(ctx, p.id, p.question, opt)
	case askAccount:
		ans, err = client.AskOnAccount(ctx, p.id, p.question, opt)
	}
	sp.Stop()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return cmdutil.NewSilentError(cmdutil.ExitTimeout,
				fmt.Errorf("AI analysis timed out after %s; try narrowing scope (fewer calls, a tighter --since)", analysisTimeout))
		}
		return err
	}

	// An ask answer is freeform prose, not a table, so the usual "non-TTY →
	// JSON" default doesn't apply: print the readable answer unless the user
	// *explicitly* asked for structured output (--json, -o <non-table>, or --jq
	// which needs JSON to filter). This keeps the guided/interactive flow and
	// plain `modjo ask call <id> "..."` human-readable while still supporting
	// `--json` for scripting.
	if f.Flags.JSON || f.Flags.JQ != "" || (f.Flags.Output != "" && f.Flags.Output != "table") {
		pr, err := f.Printer()
		if err != nil {
			return err
		}
		envelope := struct {
			Answer string `json:"answer"`
			Entity string `json:"entity"`
			Type   string `json:"type"`
		}{ans.Answer, p.id, p.name}
		return pr.PrintJSON(envelope)
	}
	fmt.Fprintln(f.IOStreams.Out, ans.Answer)
	return nil
}

func newAskSub(f *cmdutil.Factory, name string, typ askType, idHint string) *cobra.Command {
	var language string
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s %s <question>", name, idHint),
		Short: fmt.Sprintf("Ask about a %s", name),
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAsk(cmd, f, askParams{
				typ:      typ,
				name:     name,
				id:       args[0],
				question: strings.Join(args[1:], " "),
				language: language,
			})
		},
	}
	cmd.Flags().StringVar(&language, "language", "", "Response language (defaults to profile)")
	return cmd
}

// runInteractive is the guided picker shown by `modjo ask` with no arguments:
// choose the entity type, resolve it (accounts by name, calls/deals by ID),
// type a question, and run the analysis.
func runInteractive(cmd *cobra.Command, f *cmdutil.Factory) error {
	io := f.IOStreams
	types := []struct {
		name string
		typ  askType
	}{{"call", askCall}, {"deal", askDeal}, {"account", askAccount}}

	idx, err := io.Select("What do you want to ask about?", []string{"call", "deal", "account"})
	if err != nil {
		return cmdutil.NewUsageError(err)
	}
	chosen := types[idx]

	id, err := resolveEntity(cmd, f, chosen.typ, chosen.name)
	if err != nil {
		return err
	}

	question, err := io.Prompt(fmt.Sprintf("Ask about %s %s: ", chosen.name, id))
	if err != nil || strings.TrimSpace(question) == "" {
		return cmdutil.NewUsageError(fmt.Errorf("a question is required"))
	}

	return runAsk(cmd, f, askParams{typ: chosen.typ, name: chosen.name, id: id, question: question})
}

// resolveEntity turns the user's input into an entity ID. Accounts are
// searchable by name (the API supports it); calls/deals are entered directly.
func resolveEntity(cmd *cobra.Command, f *cmdutil.Factory, typ askType, name string) (string, error) {
	io := f.IOStreams
	if typ != askAccount {
		return io.Prompt(fmt.Sprintf("Enter the %s ID: ", name))
	}

	term, err := io.Prompt("Search accounts by name: ")
	if err != nil || term == "" {
		return "", cmdutil.NewUsageError(fmt.Errorf("a search term is required"))
	}
	client, err := f.APIClient()
	if err != nil {
		return "", err
	}
	labels, ids := make([]string, 0), make([]string, 0)
	for a, err := range client.Accounts(cmd.Context(), api.AccountFilter{Name: term, Limit: 10}) {
		if err != nil {
			return "", err
		}
		labels = append(labels, fmt.Sprintf("%s (%s)", a.Name, a.CRMID))
		ids = append(ids, a.CRMID)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no accounts match %q", term)
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	pick, err := io.Select("Select an account:", labels)
	if err != nil {
		return "", cmdutil.NewUsageError(err)
	}
	return ids[pick], nil
}
