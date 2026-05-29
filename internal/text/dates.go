// Package text holds small, pure string/time helpers that the rest of the CLI
// depends on. Keeping them dependency-free and well-tested matters: date
// normalization in particular is a frequent source of user error, so the spec
// calls it out as a first-class TDD target.
package text

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Clock abstracts "now" so relative-date math is deterministic under test.
type Clock interface{ Now() time.Time }

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// FixedClock returns a Clock pinned to t.
func FixedClock(t time.Time) Clock { return fixedClock{t: t} }

// SystemClock returns a Clock backed by time.Now.
func SystemClock() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func isoFromYMD(y, m, d int) string {
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

var (
	isoRe      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	relativeRe = regexp.MustCompile(`^(\d+)\s*([dwmy])$`)
)

var weekdays = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// NormalizeDate converts a user-supplied date expression into the API's strict
// YYYY-MM-DD form. It accepts ISO dates, relative offsets (30d, 2w, 6m, 1y),
// the keywords today/yesterday/tomorrow, and "last <weekday>". An empty input
// returns an empty string (the caller decides whether the field is required).
func NormalizeDate(in string, clk Clock) (string, error) {
	s := strings.TrimSpace(strings.ToLower(in))
	if s == "" {
		return "", nil
	}

	// Already ISO? Validate it really parses (catches 2026-13-40).
	if isoRe.MatchString(s) {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return "", fmt.Errorf("invalid date %q: %w", in, err)
		}
		return s, nil
	}

	now := clk.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch s {
	case "today":
		return today.Format("2006-01-02"), nil
	case "yesterday":
		return today.AddDate(0, 0, -1).Format("2006-01-02"), nil
	case "tomorrow":
		return today.AddDate(0, 0, 1).Format("2006-01-02"), nil
	}

	if m := relativeRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "d":
			return today.AddDate(0, 0, -n).Format("2006-01-02"), nil
		case "w":
			return today.AddDate(0, 0, -7*n).Format("2006-01-02"), nil
		case "m":
			return today.AddDate(0, -n, 0).Format("2006-01-02"), nil
		case "y":
			return today.AddDate(-n, 0, 0).Format("2006-01-02"), nil
		}
	}

	if strings.HasPrefix(s, "last ") {
		name := strings.TrimSpace(strings.TrimPrefix(s, "last "))
		if wd, ok := weekdays[name]; ok {
			return lastWeekday(today, wd).Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("invalid date format %q: use YYYY-MM-DD, a relative like 30d, or \"last monday\"", in)
}

// lastWeekday returns the most recent past occurrence of wd strictly before from.
func lastWeekday(from time.Time, wd time.Weekday) time.Time {
	diff := (int(from.Weekday()) - int(wd) + 7) % 7
	if diff == 0 {
		diff = 7
	}
	return from.AddDate(0, 0, -diff)
}
