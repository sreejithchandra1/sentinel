package alerter

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestHandleHostSample_IgnoresSpikes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := &models.Host{
		Name:              "web",
		Enabled:           true,
		CollectCPU:        true,
		AlertCPUEnabled:   true,
		AlertCPUThreshold: 90,
		AlertCPUAfter:     3,
		IntervalSeconds:   30,
		NotifyEmail:       true,
	}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}

	var alerts []string
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(_ *models.Monitor, alertType, _ string, _ int) error {
		alerts = append(alerts, alertType)
		return nil
	}

	now := time.Now().UTC()
	sample := func(i int, cpu float64) *models.HostSample {
		return &models.HostSample{
			HostID:      h.ID,
			CPUPercent:  &cpu,
			CollectedAt: now.Add(time.Duration(i) * 30 * time.Second),
		}
	}

	if err := a.HandleHostSample(h, sample(1, 99)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("single spike should not alert, got %v", alerts)
	}
	open, err := st.GetOpenIncident(h.ID, models.IncidentHostCPU)
	if err != nil || open != nil {
		t.Fatalf("no incident yet: %v %+v", err, open)
	}

	if err := a.HandleHostSample(h, sample(2, 95)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("two samples should not alert, got %v", alerts)
	}

	if err := a.HandleHostSample(h, sample(3, 91)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0] != "HOST_CPU" {
		t.Fatalf("alerts=%v, want [HOST_CPU]", alerts)
	}

	if err := a.HandleHostSample(h, sample(4, 99)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Fatalf("no duplicate alert, got %v", alerts)
	}

	// Dip into hysteresis band should not recover yet; need 3 samples below warning-5 (75).
	if err := a.HandleHostSample(h, sample(5, 88)); err != nil {
		t.Fatal(err)
	}
	if err := a.HandleHostSample(h, sample(6, 80)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Fatalf("at or above warning should not recover, got %v", alerts)
	}
	if err := a.HandleHostSample(h, sample(7, 70)); err != nil {
		t.Fatal(err)
	}
	if err := a.HandleHostSample(h, sample(8, 60)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Fatalf("need 3 consecutive recoveries, got %v", alerts)
	}
	if err := a.HandleHostSample(h, sample(9, 50)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || alerts[1] != "RECOVERY" {
		t.Fatalf("alerts=%v, want recovery", alerts)
	}
}

func TestHandleHostOfflineCheck_Hysteresis(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	seen := time.Now().UTC().Add(-10 * time.Minute)
	h := &models.Host{
		Name:               "web",
		Enabled:            true,
		IntervalSeconds:    30,
		AlertAfterFailures: 2,
		LastSeenAt:         &seen,
		Status:             models.HostOnline,
	}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}

	var alerts []string
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(_ *models.Monitor, alertType, _ string, _ int) error {
		alerts = append(alerts, alertType)
		return nil
	}

	now := time.Now().UTC()
	if err := a.HandleHostOfflineCheck(h, now); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("first miss should wait, got %v", alerts)
	}
	if err := a.HandleHostOfflineCheck(h, now); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0] != "HOST_OFFLINE" {
		t.Fatalf("alerts=%v", alerts)
	}

	if err := a.HandleHostIngestOnline(h, now); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || alerts[1] != "RECOVERY" {
		t.Fatalf("alerts=%v", alerts)
	}
}

func TestHostLoadThresholdsUseCPUCores(t *testing.T) {
	h := &models.Host{AlertLoadWarning: 80, AlertLoadThreshold: 0}
	if g := models.HostLoadWarning(h, 4); g != 3.2 {
		t.Fatalf("warning=%v, want 3.2", g)
	}
	if g := models.HostLoadCritical(h, 4); g != 3.6 {
		t.Fatalf("critical=%v, want 3.6", g)
	}
	if g := models.HostLoadCritical(h, 32); g != 28.8 {
		t.Fatalf("32-core critical=%v, want 28.8", g)
	}
}

func TestHandleHostSample_DiskNamesMount(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &models.Host{
		Name: "web", Enabled: true, CollectDisk: true, AlertDiskEnabled: true,
		AlertDiskWarning: 80, AlertDiskThreshold: 90, AlertDiskAfter: 1, NotifyEmail: true,
	}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(*models.Monitor, string, string, int) error { return nil }
	now := time.Now().UTC()
	err = a.HandleHostSample(h, &models.HostSample{
		HostID: h.ID, CollectedAt: now,
		Disks: []models.HostDisk{{Mount: "/", Percent: 82}, {Mount: "/backup", Percent: 97}},
	})
	if err != nil {
		t.Fatal(err)
	}
	open, err := st.GetOpenIncident(h.ID, models.IncidentHostDisk)
	if err != nil || open == nil {
		t.Fatalf("want disk incident: %v %+v", err, open)
	}
	if !strings.Contains(open.Message, "/backup") {
		t.Fatalf("message should name the mount: %s", open.Message)
	}
}

func TestHandleHostSample_AuthBurst(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &models.Host{
		Name: "web", Enabled: true, CollectSecurity: true, AlertAuthEnabled: true,
		AlertAuthThreshold: 50, NotifyEmail: true,
		Security: &models.HostSecurity{LogsReadable: true, SSHFailed5m: 40, SudoFailed5m: 15},
	}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(*models.Monitor, string, string, int) error { return nil }
	if err := a.HandleHostSample(h, &models.HostSample{HostID: h.ID, CollectedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	open, err := st.GetOpenIncident(h.ID, models.IncidentHostAuth)
	if err != nil || open == nil {
		t.Fatalf("want auth incident: %v %+v", err, open)
	}
}
