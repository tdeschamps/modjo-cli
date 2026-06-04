package text

import (
	"testing"
	"time"
)

func TestSystemClock(t *testing.T) {
	c := SystemClock()
	now := c.Now()
	if time.Since(now) > time.Minute {
		t.Errorf("SystemClock.Now() looks wrong: %v", now)
	}
}

func TestFixedClock(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := FixedClock(want).Now(); !got.Equal(want) {
		t.Errorf("FixedClock = %v want %v", got, want)
	}
}
