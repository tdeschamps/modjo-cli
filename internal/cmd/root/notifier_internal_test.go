package root

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func factoryWith(stderrTTY, quiet bool) *cmdutil.Factory {
	io, _, _, _ := iostreams.Test()
	io.SetStderrTTY(stderrTTY)
	return &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{Quiet: quiet}}
}

func TestUpdateNotifierEnabled(t *testing.T) {
	t.Setenv("MODJO_NO_UPDATE_NOTIFIER", "")
	deals := &cobra.Command{Use: "deals"}

	if !updateNotifierEnabled(factoryWith(true, false), deals) {
		t.Error("should be enabled on an interactive, non-quiet data command")
	}
	if updateNotifierEnabled(factoryWith(false, false), deals) {
		t.Error("non-TTY stderr should disable the notifier")
	}
	if updateNotifierEnabled(factoryWith(true, true), deals) {
		t.Error("--quiet should disable the notifier")
	}

	// Denylisted commands never notify (their stdout may be sourced/captured).
	for _, name := range []string{"completion", "version", "update", "info"} {
		if updateNotifierEnabled(factoryWith(true, false), &cobra.Command{Use: name}) {
			t.Errorf("%s should be denylisted from the notifier", name)
		}
	}

	// Env suppression wins.
	t.Setenv("MODJO_NO_UPDATE_NOTIFIER", "1")
	if updateNotifierEnabled(factoryWith(true, false), deals) {
		t.Error("MODJO_NO_UPDATE_NOTIFIER should disable the notifier")
	}
}

func TestPrintUpdateNoticeNoopWhenDisabled(t *testing.T) {
	f := factoryWith(false, false) // non-TTY → disabled
	_, _, _, errBuf := iostreams.Test()
	f.IOStreams.ErrOut = errBuf
	printUpdateNotice(f, &cobra.Command{Use: "deals"})
	if errBuf.Len() != 0 {
		t.Errorf("disabled notifier must print nothing, got %q", errBuf.String())
	}
}
