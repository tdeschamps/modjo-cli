package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

// SaveConfig writes the config back to its resolved path.
func (f *Factory) SaveConfig(cfg *config.Config) error {
	path := f.ConfigPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	return config.Save(path, cfg)
}

// PromptSecret reads a secret from the terminal with echo disabled when stdin
// is a TTY, falling back to a plain line read otherwise.
func PromptSecret(io *iostreams.IOStreams, prompt string) (string, error) {
	fmt.Fprint(io.ErrOut, prompt)
	if f, ok := io.StdinFile(); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(io.ErrOut)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	br := bufio.NewReader(io.In)
	line, err := br.ReadString('\n')
	return strings.TrimSpace(line), ignoreEOF(err)
}

// Confirm asks a yes/no question, returning true on "y"/"yes". If prompting is
// disabled it returns the provided default.
func Confirm(io *iostreams.IOStreams, prompt string, def bool) (bool, error) {
	if !io.CanPrompt() {
		return def, nil
	}
	suffix := " [y/N] "
	if def {
		suffix = " [Y/n] "
	}
	fmt.Fprint(io.ErrOut, prompt+suffix)
	br := bufio.NewReader(io.In)
	line, err := br.ReadString('\n')
	if err := ignoreEOF(err); err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	case "":
		return def, nil
	default:
		return false, nil
	}
}

// OpenBrowser opens url in the user's default browser (best effort).
func OpenBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// OSLookup is os.LookupEnv exposed for resolver calls.
func OSLookup(k string) (string, bool) { return os.LookupEnv(k) }

// NormalizeDateFlag normalizes a user-supplied date flag to YYYY-MM-DD using the
// factory's clock, wrapping bad formats as usage errors (exit code 2).
func NormalizeDateFlag(f *Factory, in string) (string, error) {
	out, err := text.NormalizeDate(in, f.Clock)
	if err != nil {
		return "", NewUsageError(err)
	}
	return out, nil
}

func ignoreEOF(err error) error {
	if err != nil && err.Error() == "EOF" {
		return nil
	}
	return err
}
