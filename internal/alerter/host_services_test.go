package alerter

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestParseHostServiceMessage(t *testing.T) {
	got := ParseHostServiceMessage("Watched services not active: nginx (missing), sshd (failed)")
	if len(got) != 2 || got[0].Name != "nginx" || got[0].Status != "missing" || got[1].Name != "sshd" {
		t.Fatalf("got %#v", got)
	}
}

func TestRenderAlertEmailHostServices(t *testing.T) {
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:        "HOST_SERVICE",
		Name:         "internal",
		URL:          "internal",
		Message:      "Watched services not active: nginx (missing), sshd (failed)",
		DashboardURL: "http://localhost/hosts/1",
		IncidentID:   "deadbeef",
		EventAt:      time.Now().UTC(),
	})
	for _, want := range []string{"HOST SERVICE DOWN", "Service", "Status", "nginx", "missing", "sshd", "failed"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in html", want)
		}
	}
	if strings.Contains(html, "Watched services not active: nginx (missing), sshd (failed)") {
		t.Fatal("email should render a table, not the run-on sentence")
	}
}

func TestHandleHostSample_EmptyServiceStatusDoesNotAlert(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &models.Host{
		Name: "web", Enabled: true, CollectServices: true, AlertServiceEnabled: true,
		Services: []string{"nginx", "sshd"}, NotifyEmail: true, AlertAfterFailures: 1,
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
	if err := a.HandleHostSample(h, &models.HostSample{HostID: h.ID, CollectedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("empty service snapshot should not alert, got %v", alerts)
	}
}

func TestHandleHostSample_ServiceDownAfterThreshold(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &models.Host{
		Name: "web", Enabled: true, CollectServices: true, AlertServiceEnabled: true,
		Services: []string{"nginx", "sshd"}, ServiceStatus: []models.HostServiceStatus{
			{Name: "nginx", Active: "failed"},
			{Name: "sshd", Active: "active"},
		},
		NotifyEmail: true, AlertAfterFailures: 2,
	}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	var alerts []string
	var lastMsg string
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(_ *models.Monitor, alertType, message string, _ int) error {
		alerts = append(alerts, alertType)
		lastMsg = message
		return nil
	}
	now := time.Now().UTC()
	if err := a.HandleHostSample(h, &models.HostSample{HostID: h.ID, CollectedAt: now}); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("first failed sample should wait, got %v", alerts)
	}
	if err := a.HandleHostSample(h, &models.HostSample{HostID: h.ID, CollectedAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0] != "HOST_SERVICE" {
		t.Fatalf("alerts=%v", alerts)
	}
	if !strings.Contains(lastMsg, "nginx (failed)") || !strings.Contains(lastMsg, "sshd (active)") {
		t.Fatalf("message=%q", lastMsg)
	}
}
