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

// readPasswordTTY reads a secret with echo disabled when stdin is a TTY. The
// second return reports whether stdin was actually a terminal. It is a package
// var so tests can exercise the TTY path without a pseudo-terminal.
var readPasswordTTY = func(io *iostreams.IOStreams) (string, bool, error) {
	f, ok := io.StdinFile()
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", false, nil
	}
	b, err := term.ReadPassword(int(f.Fd()))
	return string(b), true, err
}

// PromptSecret reads a secret from the terminal with echo disabled when stdin
// is a TTY, falling back to a plain line read otherwise.
func PromptSecret(io *iostreams.IOStreams, prompt string) (string, error) {
	fmt.Fprint(io.ErrOut, prompt)
	if s, isTTY, err := readPasswordTTY(io); isTTY {
		fmt.Fprintln(io.ErrOut)
		if err != nil {
			return "", err
		}
		return s, nil
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

// browserCommand returns the command and args to open url on the given OS. It
// is split out from OpenBrowser so the platform mapping is unit-testable
// without actually launching a browser.
func browserCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}

// BrowserRunner actually starts the command; overridable in tests.
var BrowserRunner = func(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

// OpenBrowser opens url in the user's default browser (best effort).
func OpenBrowser(url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	return BrowserRunner(name, args...)
}

// OpenResource opens a resource's web link in the browser, or returns a uniform
// error when the resource has no link. Shared by the `open` subcommands.
func OpenResource(io *iostreams.IOStreams, noun, id, link string) error {
	if link == "" {
		return fmt.Errorf("%s %s has no CRM link", noun, id)
	}
	io.Errf("Opening %s\n", link)
	return OpenBrowser(link)
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
