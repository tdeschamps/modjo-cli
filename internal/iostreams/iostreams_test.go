package iostreams

import (
	"os"
	"strings"
	"testing"
)

func TestSystem(t *testing.T) {
	s := System()
	if s.In == nil || s.Out == nil || s.ErrOut == nil {
		t.Fatal("System streams must be set")
	}
}

func TestTestStreams(t *testing.T) {
	s, in, out, errOut := Test()
	in.WriteString("hi")
	b, err := s.ReadAllStdin()
	if err != nil || string(b) != "hi" {
		t.Errorf("ReadAllStdin = %q, %v", b, err)
	}
	if s.CanPrompt() {
		t.Error("Test streams should not prompt by default")
	}
	out.WriteString("o")
	errOut.WriteString("e")
}

func TestTTYAndColorSetters(t *testing.T) {
	s, _, _, _ := Test()
	s.SetStdoutTTY(true)
	if !s.IsStdoutTTY() {
		t.Error("SetStdoutTTY")
	}
	s.SetColorEnabled(true)
	if !s.ColorEnabled() {
		t.Error("SetColorEnabled")
	}
	s.SetNeverPrompt(false)
	if !s.CanPrompt() {
		t.Error("SetNeverPrompt(false) should allow prompts")
	}
	// IsStderrTTY just returns the field; exercise it.
	_ = s.IsStderrTTY()
}

func TestStdinFile(t *testing.T) {
	s, _, _, _ := Test()
	if _, ok := s.StdinFile(); ok {
		t.Error("buffer should not be an *os.File")
	}
	s.In = os.Stdin
	if _, ok := s.StdinFile(); !ok {
		t.Error("os.Stdin should be an *os.File")
	}
}

func TestColorHelpers(t *testing.T) {
	s, _, _, _ := Test()
	// Color disabled → plain passthrough.
	if s.Bold("x") != "x" || s.Red("x") != "x" || s.Green("x") != "x" ||
		s.Yellow("x") != "x" || s.Cyan("x") != "x" || s.Gray("x") != "x" {
		t.Error("color helpers should be plain when disabled")
	}
	// Color enabled → wrapped in escape codes.
	s.SetColorEnabled(true)
	for name, got := range map[string]string{
		"bold":   s.Bold("x"),
		"red":    s.Red("x"),
		"green":  s.Green("x"),
		"yellow": s.Yellow("x"),
		"cyan":   s.Cyan("x"),
		"gray":   s.Gray("x"),
	} {
		if !strings.Contains(got, "\033[") || !strings.HasSuffix(got, codeReset) {
			t.Errorf("%s not colorized: %q", name, got)
		}
	}
}

func TestErrf(t *testing.T) {
	s, _, _, errOut := Test()
	s.Errf("hello %s\n", "world")
	if errOut.String() != "hello world\n" {
		t.Errorf("Errf = %q", errOut.String())
	}
}

func TestEnvColorAllowed(t *testing.T) {
	// Ensure a clean baseline: NO_COLOR present at all (even empty) disables.
	t.Setenv("NO_COLOR", "x")
	os.Unsetenv("NO_COLOR")
	t.Setenv("MODJO_NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	if !envColorAllowed() {
		t.Error("color should be allowed by default")
	}
	t.Setenv("NO_COLOR", "1")
	if envColorAllowed() {
		t.Error("NO_COLOR should disable color")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("MODJO_NO_COLOR", "1")
	if envColorAllowed() {
		t.Error("MODJO_NO_COLOR should disable color")
	}
	t.Setenv("MODJO_NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if envColorAllowed() {
		t.Error("TERM=dumb should disable color")
	}
}
