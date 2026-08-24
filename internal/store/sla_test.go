package store

import (
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestOverlapSeconds(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	if got := overlapSeconds(t0, t2, t1, t2); got != 3600 {
		t.Fatalf("got %d, want 3600", got)
	}
	if got := overlapSeconds(t0, t1, t1, t2); got != 0 {
		t.Fatalf("touching ranges got %d, want 0", got)
	}
}

func TestBuildSLAReport_MaintenanceAndMTTR(t *testing.T) {
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	window := int64(to.Sub(from) / time.Second)

	aID, bID := "mon-a", "mon-b"
	monitors := []models.MonitorListItem{
		{Monitor: models.Monitor{ID: aID, Name: "A"}},
		{Monitor: models.Monitor{ID: bID, Name: "B"}},
	}
	start := from.Add(24 * time.Hour)
	end := start.Add(time.Hour)
	incidents := []models.IncidentListItem{
		{Incident: models.Incident{
			ID: "i1", MonitorID: aID, Type: models.IncidentDown,
			StartedAt: start, ResolvedAt: &end,
		}, MonitorName: "A"},
	}
	windows := []models.MaintenanceWindow{
		{MonitorID: aID, StartsAt: start, EndsAt: start.Add(30 * time.Minute)},
	}

	rep := BuildSLAReport(monitors, incidents, windows, from, to)
	if rep.IncidentCount != 1 {
		t.Fatalf("incidents=%d", rep.IncidentCount)
	}
	if rep.Monitors[0].DowntimeSeconds != 1800 {
		t.Fatalf("downtime=%d, want 1800 after maintenance", rep.Monitors[0].DowntimeSeconds)
	}
	if rep.Monitors[1].DowntimeSeconds != 0 {
		t.Fatalf("monitor B should have no downtime")
	}
	if rep.Monitors[0].MTTRSeconds == nil || *rep.Monitors[0].MTTRSeconds != 3600 {
		t.Fatalf("mttr=%v, want 3600", rep.Monitors[0].MTTRSeconds)
	}
	wantA := (1 - 1800/float64(window)) * 100
	if abs(rep.Monitors[0].AvailabilityPct-wantA) > 0.0001 {
		t.Fatalf("avail A=%v want %v", rep.Monitors[0].AvailabilityPct, wantA)
	}
	avg := (wantA + 100) / 2
	if abs(rep.AvailabilityPct-avg) > 0.0001 {
		t.Fatalf("fleet avail=%v want %v", rep.AvailabilityPct, avg)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
