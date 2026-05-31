package iostreams

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// inputReader returns a single buffered reader over stdin, created lazily.
// Reusing one reader across prompts is essential: a fresh bufio.Reader per call
// would read ahead and discard buffered bytes, dropping subsequent answers.
func (s *IOStreams) inputReader() *bufio.Reader {
	if s.inReader == nil {
		s.inReader = bufio.NewReader(s.In)
	}
	return s.inReader
}

// Prompt writes label to stderr and reads a trimmed line from stdin. It is for
// interactive use; callers should gate on CanPrompt first.
func (s *IOStreams) Prompt(label string) (string, error) {
	fmt.Fprint(s.ErrOut, label)
	line, err := s.inputReader().ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Select prints a numbered list of options to stderr and reads the user's
// 1-based choice from stdin, returning the 0-based index. It re-reads is the
// caller's job; here a bad choice is an error.
func (s *IOStreams) Select(label string, options []string) (int, error) {
	fmt.Fprintln(s.ErrOut, s.Bold(label))
	for i, opt := range options {
		fmt.Fprintf(s.ErrOut, "  %s %s\n", s.Cyan(strconv.Itoa(i+1)), opt)
	}
	raw, err := s.Prompt("Choice: ")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > len(options) {
		return 0, fmt.Errorf("invalid choice %q", raw)
	}
	return n - 1, nil
}
