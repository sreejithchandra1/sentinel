package api

import (
	"testing"
	"time"
)

func TestParseSLAMonth(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	from, to, err := parseSLAMonth("2026-07", now)
	if err != nil {
		t.Fatal(err)
	}
	if from.Month() != time.July || to != time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("july: from=%v to=%v", from, to)
	}
	from, to, err = parseSLAMonth("2026-08", now)
	if err != nil {
		t.Fatal(err)
	}
	if !to.Equal(now) {
		t.Fatalf("current month end should be now, got %v", to)
	}
	if _, _, err := parseSLAMonth("nope", now); err == nil {
		t.Fatal("expected invalid month")
	}
}
