package iostreams

import (
	"fmt"
	"os"
)

// Hyperlink renders text as an OSC 8 terminal hyperlink when stdout is an
// interactive terminal that plausibly supports it. Elsewhere (pipes, dumb
// terminals, or when the displayed text already is the URL) it degrades to
// plain text so output stays clean and copy-pasteable.
func (s *IOStreams) Hyperlink(text, url string) string {
	if url == "" {
		return text
	}
	if text == "" {
		text = url
	}
	if !s.stdoutTTY || !hyperlinksSupported() || text == url {
		return text
	}
	// OSC 8 ; params ; URL ST  <text>  OSC 8 ; ; ST
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// hyperlinksSupported is a conservative check: most modern terminals support
// OSC 8, but "dumb" terminals and CI do not.
func hyperlinksSupported() bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	return true
}
