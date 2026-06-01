package calls

import (
	"testing"
	"time"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/text"
)

func TestSplitCSV(t *testing.T) {
	if splitCSV("") != nil {
		t.Error("empty → nil")
	}
	got := splitCSV(" a , b ,, c ")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("splitCSV = %v", got)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 60) != "short" {
		t.Error("short passthrough")
	}
	if truncate("multi\nline", 60) != "multi line" {
		t.Error("newlines collapsed")
	}
	got := truncate("abcdefghij", 5)
	if len([]rune(got)) != 5 {
		t.Errorf("truncate length = %q", got)
	}
}

func TestFmtTime(t *testing.T) {
	if fmtTime(75) != "01:15" {
		t.Errorf("fmtTime(75) = %q", fmtTime(75))
	}
	if got := fmtTime(3725); got != "01:02:05" { // 1h 2m 5s — must not wrap to 62:05
		t.Errorf("fmtTime(3725) = %q want 01:02:05", got)
	}
	if got := fmtTime(4530); got != "01:15:30" {
		t.Errorf("fmtTime(4530) = %q want 01:15:30", got)
	}
}

func TestBuildFilter(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams:  io,
		Flags:      &cmdutil.GlobalFlags{},
		Clock:      text.FixedClock(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)),
		ConfigPath: t.TempDir() + "/c.toml",
	}
	flt, err := buildFilter(f, &listFlags{account: "a", since: "30d", until: "2026-05-29", expand: "deal,account"})
	if err != nil {
		t.Fatal(err)
	}
	if flt.Account != "a" || flt.Since != "2026-04-29" || len(flt.Expand) != 2 {
		t.Errorf("filter = %+v", flt)
	}

	// Bad since date.
	if _, err := buildFilter(f, &listFlags{since: "05/01/2026"}); err == nil {
		t.Error("bad since should error")
	}
	// Bad until date.
	if _, err := buildFilter(f, &listFlags{until: "nope"}); err == nil {
		t.Error("bad until should error")
	}
}
