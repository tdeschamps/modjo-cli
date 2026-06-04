package text

import (
	"testing"
	"time"
)

func TestNormalizeDateAllForms(t *testing.T) {
	clk := FixedClock(mustDate("2026-05-29")) // a Friday
	cases := map[string]string{
		"":            "",
		"today":       "2026-05-29",
		"tomorrow":    "2026-05-30",
		"yesterday":   "2026-05-28",
		"7d":          "2026-05-22",
		"2w":          "2026-05-15",
		"3m":          "2026-03-01", // Feb 29 2026 doesn't exist → normalized
		"1y":          "2025-05-29",
		"last friday": "2026-05-22", // diff==0 → 7 days back
		"last sunday": "2026-05-24",
	}
	for in, want := range cases {
		got, err := NormalizeDate(in, clk)
		if err != nil {
			t.Errorf("NormalizeDate(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeDate(%q) = %q want %q", in, got, want)
		}
	}
}

func TestNormalizeDateErrors(t *testing.T) {
	clk := FixedClock(mustDate("2026-05-29"))
	for _, in := range []string{"2026-13-40", "05/01/2026", "last someday", "nope", "5x"} {
		if _, err := NormalizeDate(in, clk); err == nil {
			t.Errorf("NormalizeDate(%q) should error", in)
		}
	}
}

func TestMustDatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustDate should panic on bad input")
		}
	}()
	_ = mustDate("not-a-date")
}

func TestLastWeekdaySameDay(t *testing.T) {
	// from is Monday; "last monday" should go back a full week.
	monday := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	got := lastWeekday(monday, time.Monday)
	if !got.Equal(monday.AddDate(0, 0, -7)) {
		t.Errorf("lastWeekday same-day = %v", got)
	}
}
