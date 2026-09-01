package alerter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestHandlePerformanceResult_HysteresisAndFailures(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := &models.PerformanceTarget{
		Name:            "site",
		URL:             "https://example.com",
		IntervalSeconds: 60,
		TimeoutMs:       10000,
		SlowThresholdMs: 3000,
		Enabled:         true,
		AlertAfterSlow:  2,
		LastStatus:      models.StatusUp,
	}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	now := time.Now().UTC()
	tick := 0
	apply := func(status models.MonitorStatus) {
		t.Helper()
		prev := target.LastStatus
		if status == models.StatusDegraded {
			target.ConsecutiveSlow++
		} else {
			target.ConsecutiveSlow = 0
		}
		tick++
		if err := a.HandlePerformanceResult(target, &models.PerformanceResult{
			TargetID:       target.ID,
			Status:         status,
			ResponseTimeMs: 4000,
			CheckedAt:      now.Add(time.Duration(tick) * time.Minute),
		}, prev); err != nil {
			t.Fatal(err)
		}
		target.LastStatus = status
	}
	openSlow := func() bool {
		t.Helper()
		inc, err := st.GetOpenIncident(target.ID, models.IncidentSlow)
		if err != nil {
			t.Fatal(err)
		}
		return inc != nil
	}

	apply(models.StatusDegraded)
	if openSlow() {
		t.Fatal("one slow check must not open a SLOW incident")
	}

	apply(models.StatusDegraded)
	if !openSlow() {
		t.Fatal("two consecutive slow checks should open a SLOW incident")
	}

	apply(models.StatusDegraded)
	if !openSlow() {
		t.Fatal("further slow checks should keep the incident open")
	}

	apply(models.StatusUp)
	if !openSlow() {
		t.Fatal("one OK check must not resolve yet")
	}

	apply(models.StatusUp)
	if openSlow() {
		t.Fatal("two consecutive OK checks should send NORMAL and resolve")
	}

	apply(models.StatusDown)
	apply(models.StatusDown)
	if openSlow() {
		t.Fatal("failed probes must not open a SLOW incident")
	}

	apply(models.StatusDegraded)
	apply(models.StatusDown)
	apply(models.StatusDegraded)
	if openSlow() {
		t.Fatal("a failure must reset the slow streak")
	}

	apply(models.StatusDegraded)
	apply(models.StatusDegraded)
	if !openSlow() {
		t.Fatal("want SLOW incident after two slow checks")
	}
	apply(models.StatusDown)
	if !openSlow() {
		t.Fatal("a failure during a slow incident must not resolve it")
	}
	apply(models.StatusUp)
	if !openSlow() {
		t.Fatal("recovery streak should reset after a failure")
	}
	apply(models.StatusUp)
	if openSlow() {
		t.Fatal("two OK checks after the failure should resolve")
	}
}

func TestHandlePerformanceResult_DefaultThresholdIsTwo(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := &models.PerformanceTarget{
		Name:            "site",
		URL:             "https://example.com",
		Enabled:         true,
		AlertAfterSlow:  0,
		SlowThresholdMs: 3000,
	}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}
	if target.AlertAfterSlow != 2 {
		t.Fatalf("create default alert_after_slow=%d want 2", target.AlertAfterSlow)
	}

	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	target.AlertAfterSlow = 0
	target.ConsecutiveSlow = 1
	if err := a.HandlePerformanceResult(target, &models.PerformanceResult{
		TargetID:  target.ID,
		Status:    models.StatusDegraded,
		CheckedAt: time.Now().UTC(),
	}, models.StatusUp); err != nil {
		t.Fatal(err)
	}
	inc, err := st.GetOpenIncident(target.ID, models.IncidentSlow)
	if err != nil {
		t.Fatal(err)
	}
	if inc != nil {
		t.Fatal("AlertAfterSlow 0 should clamp to 2, so one slow check must not alert")
	}

	target.ConsecutiveSlow = 2
	if err := a.HandlePerformanceResult(target, &models.PerformanceResult{
		TargetID:  target.ID,
		Status:    models.StatusDegraded,
		CheckedAt: time.Now().UTC(),
	}, models.StatusDegraded); err != nil {
		t.Fatal(err)
	}
	inc, err = st.GetOpenIncident(target.ID, models.IncidentSlow)
	if err != nil {
		t.Fatal(err)
	}
	if inc == nil {
		t.Fatal("second slow check should alert when threshold clamps to 2")
	}
}
