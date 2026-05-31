package iostreams

import (
	"strings"
	"testing"
	"time"
)

func TestBannerKinds(t *testing.T) {
	for _, kind := range []BannerKind{BannerInfo, BannerWarn, BannerError} {
		s, _, _, errOut := Test()
		s.RenderBanner(Banner{Kind: kind, Headline: "Heads up"})
		if !strings.Contains(errOut.String(), "Heads up") {
			t.Errorf("kind %d missing headline: %q", kind, errOut.String())
		}
	}
}

func TestSpinnerIdempotent(t *testing.T) {
	s, _, _, _ := Test()
	s.stderrTTY = true
	s.SetProgressEnabled(true)
	sp := s.NewSpinner("x")
	sp.Start()
	sp.Start() // second start is a no-op (already active)
	sp.Stop()
	sp.Stop() // second stop is a no-op (already stopped)
}

func TestSpinnerAnimates(t *testing.T) {
	s, _, _, errOut := Test()
	s.stderrTTY = true
	s.SetProgressEnabled(true)
	sp := s.NewSpinner("loading")
	sp.interval = time.Millisecond
	sp.Start()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		sp.Update("loading…")
		time.Sleep(2 * time.Millisecond)
	}
	sp.Stop()
	if frames := strings.Count(errOut.String(), "\r"); frames < 2 {
		t.Errorf("expected multiple painted frames, got %d", frames)
	}
}

func TestSymbolsPlainVsColor(t *testing.T) {
	s, _, _, _ := Test()
	// No color → ASCII fallbacks, no escape codes.
	for _, got := range []string{s.SuccessIcon(), s.ErrorIcon(), s.WarnIcon(), s.InfoIcon()} {
		if strings.Contains(got, "\033") {
			t.Errorf("icon should be plain without color: %q", got)
		}
	}
	if s.SuccessIcon() != "v" || s.ErrorIcon() != "x" {
		t.Errorf("ascii fallbacks wrong: %q %q", s.SuccessIcon(), s.ErrorIcon())
	}
	s.SetColorEnabled(true)
	if !strings.Contains(s.SuccessIcon(), "✓") || !strings.Contains(s.SuccessIcon(), "\033") {
		t.Errorf("colored success icon: %q", s.SuccessIcon())
	}
}

func TestStatusColor(t *testing.T) {
	s, _, _, _ := Test()
	// No color → passthrough.
	if s.StatusColor("Open") != "Open" {
		t.Errorf("no-color status should pass through")
	}
	s.SetColorEnabled(true)
	for _, status := range []string{"Open", "Closed won", "Closed lost", "Closed", "Deleted"} {
		if !strings.Contains(s.StatusColor(status), "\033") {
			t.Errorf("status %q should be colorized", status)
		}
	}
	if s.StatusColor("") != "" {
		t.Error("empty status stays empty")
	}
	if strings.Contains(s.StatusColor("Weird"), "\033") {
		t.Error("unknown status should not be colorized")
	}
}

func TestHyperlink(t *testing.T) {
	s, _, _, _ := Test()
	// Not a TTY → plain text, no escape.
	if got := s.Hyperlink("Docs", "https://x"); got != "Docs" {
		t.Errorf("non-TTY hyperlink = %q", got)
	}
	// Empty url → text unchanged.
	if got := s.Hyperlink("Docs", ""); got != "Docs" {
		t.Errorf("empty url = %q", got)
	}
	// TTY → OSC 8 sequence wrapping the text.
	s.SetStdoutTTY(true)
	t.Setenv("TERM", "xterm")
	t.Setenv("CI", "")
	got := s.Hyperlink("Docs", "https://x")
	if !strings.Contains(got, "\033]8;;https://x") || !strings.Contains(got, "Docs") {
		t.Errorf("TTY hyperlink missing OSC 8: %q", got)
	}
	// text == url → no link wrapper (already copy-pasteable).
	if s.Hyperlink("https://x", "https://x") != "https://x" {
		t.Error("text==url should not be wrapped")
	}
}

func TestBannerRendersToStderr(t *testing.T) {
	s, _, out, errOut := Test()
	s.RenderBanner(Banner{
		Kind:      BannerSuccess,
		Headline:  "Logged in",
		Body:      "workspace acme-eu",
		NextSteps: []string{"Run modjo calls list"},
		Links:     []string{"https://cli.modjo.ai/docs"},
	})
	if out.Len() != 0 {
		t.Errorf("banner must not write to stdout, got %q", out.String())
	}
	e := errOut.String()
	for _, want := range []string{"Logged in", "workspace acme-eu", "Next steps:", "Run modjo calls list", "Links:", "cli.modjo.ai"} {
		if !strings.Contains(e, want) {
			t.Errorf("banner missing %q:\n%s", want, e)
		}
	}
}

func TestSpinnerDisabledIsNoop(t *testing.T) {
	s, _, _, errOut := Test() // progressEnabled defaults false; not a TTY
	sp := s.NewSpinner("working…")
	sp.Start()
	sp.Update("still working…")
	sp.Stop()
	if errOut.Len() != 0 {
		t.Errorf("disabled spinner must produce no output, got %q", errOut.String())
	}
}

func TestSpinnerEnabledWritesAndClears(t *testing.T) {
	s, _, _, errOut := Test()
	s.stderrTTY = true
	s.SetProgressEnabled(true)
	sp := s.NewSpinner("loading")
	sp.Start()
	sp.Stop()
	got := errOut.String()
	if !strings.Contains(got, "loading") {
		t.Errorf("spinner should paint the message: %q", got)
	}
	// Stop clears the line with a carriage return.
	if !strings.Contains(got, "\r") {
		t.Errorf("spinner stop should clear the line: %q", got)
	}
}
