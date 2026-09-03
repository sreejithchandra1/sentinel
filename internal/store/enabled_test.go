package store

import (
	"path/filepath"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestSetMonitorEnabled_KeepsConfigAndSkipsScheduler(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:        "web",
		URL:         "https://example.com",
		Enabled:     true,
		AlertEmails: "ops@example.com",
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateMonitorState(m.ID, models.StatusDown, 3, m.UpdatedAt); err != nil {
		t.Fatal(err)
	}

	if err := st.SetMonitorEnabled(m.ID, false); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetMonitor(m.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	if got.Enabled {
		t.Fatal("expected paused")
	}
	if got.Name != "web" || got.URL != "https://example.com" || got.AlertEmails != "ops@example.com" {
		t.Fatalf("config wiped: %+v", got)
	}
	if got.LastStatus != models.StatusDown || got.ConsecutiveFailures != 3 {
		t.Fatalf("state changed: status=%s failures=%d", got.LastStatus, got.ConsecutiveFailures)
	}

	enabled, err := st.ListEnabledMonitors()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range enabled {
		if item.ID == m.ID {
			t.Fatal("paused monitor still listed as enabled")
		}
	}

	if err := st.SetMonitorEnabled(m.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetMonitor(m.ID)
	if err != nil || got == nil || !got.Enabled {
		t.Fatalf("resume: %v %v", got, err)
	}
	enabled, err = st.ListEnabledMonitors()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range enabled {
		if item.ID == m.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("resumed monitor missing from enabled list")
	}
}

func TestSetPerformanceTargetEnabled_KeepsConfigAndSkipsScheduler(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := &models.PerformanceTarget{
		Name:            "api",
		URL:             "https://api.example.com",
		Enabled:         true,
		SlowThresholdMs: 800,
	}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}

	if err := st.SetPerformanceTargetEnabled(target.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPerformanceTarget(target.ID)
	if err != nil || got == nil || got.Enabled {
		t.Fatalf("paused: %v %v", got, err)
	}
	if got.Name != "api" || got.SlowThresholdMs != 800 {
		t.Fatalf("config wiped: %+v", got)
	}

	enabled, err := st.ListEnabledPerformanceTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range enabled {
		if item.ID == target.ID {
			t.Fatal("paused target still listed as enabled")
		}
	}

	if err := st.SetPerformanceTargetEnabled(target.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetPerformanceTarget(target.ID)
	if err != nil || got == nil || !got.Enabled {
		t.Fatalf("resume: %v %v", got, err)
	}
}
