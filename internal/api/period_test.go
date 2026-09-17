package api

import (
	"testing"
	"time"
)

func TestParseStatsPeriod(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		in      string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{"", 24*time.Hour - time.Minute, 24*time.Hour + time.Minute},
		{"24h", 24*time.Hour - time.Minute, 24*time.Hour + time.Minute},
		{"7d", 7*24*time.Hour - time.Hour, 7*24*time.Hour + time.Hour},
		{"15m", 15*time.Minute - time.Second, 15*time.Minute + time.Second},
		{"1h", time.Hour - time.Second, time.Hour + time.Second},
		{"2w", 14*24*time.Hour - time.Hour, 14*24*time.Hour + time.Hour},
		{"90d", 90*24*time.Hour - time.Hour, 90*24*time.Hour + time.Hour},
		{"bogus", 24*time.Hour - time.Minute, 24*time.Hour + time.Minute},
	}
	for _, tc := range cases {
		got := now.Sub(relativeSince(normalizeStatsPeriod(tc.in), now))
		if got < tc.wantMin || got > tc.wantMax {
			t.Fatalf("period %q span=%v, want %v..%v", tc.in, got, tc.wantMin, tc.wantMax)
		}
	}
}

func TestRelativeSinceMonths(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	got := relativeSince("3M", now)
	want := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("3M from %v = %v, want %v", now, got, want)
	}
}

func TestParseStatsRangeAbsolute(t *testing.T) {
	cases := [][2]string{
		{"2026-09-16T14:00:00Z", "2026-09-16T15:00:00Z"},
		{"2026-09-16T14:00:00.000Z", "2026-09-16T15:00:00.123Z"},
		{"2026-09-16 14:00", "2026-09-16 15:00"},
	}
	for _, tc := range cases {
		win := parseStatsRange("", tc[0], tc[1])
		if win.Period != "custom" {
			t.Fatalf("%s period=%q", tc[0], win.Period)
		}
		if got := win.To.Sub(win.From); got < time.Hour-time.Second || got > time.Hour+time.Second {
			t.Fatalf("%s span=%v", tc[0], got)
		}
	}
}

func TestParseStatsRangeInvalidAbsoluteFallsBack(t *testing.T) {
	win := parseStatsRange("6h", "nope", "")
	if win.Period != "6h" {
		t.Fatalf("period=%q", win.Period)
	}
	if win.To.Sub(win.From) < 5*time.Hour || win.To.Sub(win.From) > 7*time.Hour {
		t.Fatalf("span=%v", win.To.Sub(win.From))
	}
}

func TestNormalizeStatsPeriod(t *testing.T) {
	if got := normalizeStatsPeriod(""); got != "24h" {
		t.Fatalf("empty=%q", got)
	}
	if got := normalizeStatsPeriod("45m"); got != "45m" {
		t.Fatalf("custom=%q", got)
	}
	if got := normalizeStatsPeriod("2w"); got != "2w" {
		t.Fatalf("weeks=%q", got)
	}
	if got := normalizeStatsPeriod("3M"); got != "3M" {
		t.Fatalf("months=%q", got)
	}
}
