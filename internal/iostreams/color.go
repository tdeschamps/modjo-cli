package iostreams

import "fmt"

// ANSI color helpers. These respect ColorEnabled so non-TTY / NO_COLOR output
// stays plain — important so we never break someone's awk/grep pipeline.

const (
	codeReset  = "\033[0m"
	codeBold   = "\033[1m"
	codeRed    = "\033[31m"
	codeGreen  = "\033[32m"
	codeYellow = "\033[33m"
	codeCyan   = "\033[36m"
	codeGray   = "\033[90m"
)

func (s *IOStreams) colorize(code, str string) string {
	if !s.colorEnabled {
		return str
	}
	return code + str + codeReset
}

// Bold renders text bold when color is enabled.
func (s *IOStreams) Bold(str string) string { return s.colorize(codeBold, str) }

// Red renders text red when color is enabled.
func (s *IOStreams) Red(str string) string { return s.colorize(codeRed, str) }

// Green renders text green when color is enabled.
func (s *IOStreams) Green(str string) string { return s.colorize(codeGreen, str) }

// Yellow renders text yellow when color is enabled.
func (s *IOStreams) Yellow(str string) string { return s.colorize(codeYellow, str) }

// Cyan renders text cyan when color is enabled.
func (s *IOStreams) Cyan(str string) string { return s.colorize(codeCyan, str) }

// Gray renders text gray when color is enabled.
func (s *IOStreams) Gray(str string) string { return s.colorize(codeGray, str) }

// Errf writes a formatted line to ErrOut.
func (s *IOStreams) Errf(format string, a ...any) {
	fmt.Fprintf(s.ErrOut, format, a...)
}
