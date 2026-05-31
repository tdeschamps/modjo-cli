// Package iostreams provides testable stdin/stdout/stderr handling with TTY
// and color detection. The pattern is borrowed from cli/go-gh: by injecting an
// IOStreams value into commands instead of reaching for os.Stdout directly, we
// can assert on rendered bytes in tests and drive TTY/no-TTY behavior
// deterministically.
package iostreams

import (
	"bytes"
	"io"
	"os"

	"golang.org/x/term"
)

// IOStreams bundles the three standard streams plus presentation state.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	stdoutTTY    bool
	stderrTTY    bool
	colorEnabled bool

	// neverPrompt disables interactive prompting (e.g. when --quiet or no TTY).
	neverPrompt bool

	// progressEnabled gates spinners/progress (off when piped, --quiet, or
	// --hide-spinner). It governs chrome only — never data on stdout.
	progressEnabled bool
}

// System returns IOStreams wired to the real process streams, with TTY and
// color auto-detected.
func System() *IOStreams {
	stdoutTTY := isTerminal(os.Stdout)
	stderrTTY := isTerminal(os.Stderr)
	return &IOStreams{
		In:              os.Stdin,
		Out:             os.Stdout,
		ErrOut:          os.Stderr,
		stdoutTTY:       stdoutTTY,
		stderrTTY:       stderrTTY,
		colorEnabled:    stdoutTTY && envColorAllowed(),
		neverPrompt:     !stdoutTTY,
		progressEnabled: stderrTTY,
	}
}

// Test returns an IOStreams backed by in-memory buffers, for assertions. It
// returns the streams plus the in/out/err buffers.
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:          in,
		Out:         out,
		ErrOut:      errOut,
		neverPrompt: true,
	}, in, out, errOut
}

// ReadAllStdin reads everything available on In.
func (s *IOStreams) ReadAllStdin() ([]byte, error) {
	return io.ReadAll(s.In)
}

// StdinFile returns In as an *os.File when possible (for password prompts).
func (s *IOStreams) StdinFile() (*os.File, bool) {
	f, ok := s.In.(*os.File)
	return f, ok
}

// IsStdoutTTY reports whether stdout is an interactive terminal.
func (s *IOStreams) IsStdoutTTY() bool { return s.stdoutTTY }

// IsStderrTTY reports whether stderr is an interactive terminal.
func (s *IOStreams) IsStderrTTY() bool { return s.stderrTTY }

// SetStdoutTTY overrides TTY detection (tests and --color handling).
func (s *IOStreams) SetStdoutTTY(v bool) { s.stdoutTTY = v }

// SetStderrTTY overrides stderr TTY detection (tests and presentation).
func (s *IOStreams) SetStderrTTY(v bool) { s.stderrTTY = v }

// ColorEnabled reports whether ANSI color should be emitted.
func (s *IOStreams) ColorEnabled() bool { return s.colorEnabled }

// SetColorEnabled forces color on or off.
func (s *IOStreams) SetColorEnabled(v bool) { s.colorEnabled = v }

// CanPrompt reports whether interactive prompts are allowed.
func (s *IOStreams) CanPrompt() bool { return !s.neverPrompt }

// SetNeverPrompt disables interactive prompting.
func (s *IOStreams) SetNeverPrompt(v bool) { s.neverPrompt = v }

// ProgressEnabled reports whether spinners/progress chrome should render.
func (s *IOStreams) ProgressEnabled() bool { return s.progressEnabled }

// SetProgressEnabled toggles spinners/progress chrome.
func (s *IOStreams) SetProgressEnabled(v bool) { s.progressEnabled = v }

func envColorAllowed() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("MODJO_NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
