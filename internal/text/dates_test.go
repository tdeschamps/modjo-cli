package text

import "testing"

func date(y int, m, d int) string {
	return isoFromYMD(y, m, d)
}

func TestNormalizeDate(t *testing.T) {
	clk := FixedClock(mustDate("2026-05-29"))
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"iso passthrough", "2026-05-01", "2026-05-01", false},
		{"relative days", "30d", "2026-04-29", false},
		{"relative weeks", "2w", "2026-05-15", false},
		{"relative today", "today", "2026-05-29", false},
		{"relative yesterday", "yesterday", "2026-05-28", false},
		{"natural last monday", "last monday", "2026-05-25", false},
		{"empty", "", "", false},
		{"bad format", "05/01/2026", "", true},
		{"garbage", "not-a-date", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeDate(tc.in, clk)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeDate_DateOnceMore(t *testing.T) {
	if date(2026, 5, 25) != "2026-05-25" {
		t.Fatal("isoFromYMD broken")
	}
}
