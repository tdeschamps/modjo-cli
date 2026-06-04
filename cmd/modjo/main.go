// Command modjo is the entrypoint: it builds the root command, executes it, and
// maps errors to process exit codes (product spec §9).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmd/root"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	io := iostreams.System()
	flags := &cmdutil.GlobalFlags{}
	factory := cmdutil.New(io, flags)

	rootCmd := root.NewCmdRoot(factory)
	rootCmd.SetArgs(os.Args[1:])

	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return cmdutil.ExitOK
	}

	printError(io, err)
	return cmdutil.ExitCodeForError(err)
}

// printError renders an actionable, structured error to stderr.
func printError(io *iostreams.IOStreams, err error) {
	var silent *cmdutil.SilentError
	if errors.As(err, &silent) {
		// Already reported by the command; print only the message.
		fmt.Fprintln(io.ErrOut, io.Red("Error:"), silent.Error())
		return
	}

	fmt.Fprintln(io.ErrOut, io.Red("Error:"), err.Error())

	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		if hint := remediation(apiErr.StatusCode); hint != "" {
			fmt.Fprintln(io.ErrOut, "Hint: ", hint)
		}
		if apiErr.RequestID != "" {
			fmt.Fprintf(io.ErrOut, "Request ID: %s  (run with --debug for details)\n", apiErr.RequestID)
		}
	}
}

func remediation(status int) string {
	switch status {
	case 401:
		return "authentication failed — run `modjo auth login` or regenerate your key at Settings → Integrations"
	case 403:
		return "your token lacks the required scope for this operation"
	case 404:
		return "the requested resource was not found — check the ID"
	case 422:
		return "a filter or date was invalid — use YYYY-MM-DD (or a relative like 30d)"
	case 429:
		return "rate limited — retries were exhausted; try again shortly or lower --max-retries pressure"
	}
	return ""
}
