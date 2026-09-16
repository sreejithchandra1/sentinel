package alerter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAlertMetaLabels(t *testing.T) {
	started := time.Date(2026, 8, 11, 10, 13, 0, 0, time.UTC)
	meta := AlertMeta{
		Event:      "RECOVERY",
		Name:       "Google",
		IncidentID: "abcdef12-zzzz",
		ResponseMs: 168,
		EventAt:    started.Add(2*time.Minute + 14*time.Second),
		StartedAt:  &started,
	}
	if meta.Title() != "Recovered after 2 minutes" {
		t.Fatalf("title=%q", meta.Title())
	}
	if meta.FallbackText() != "Recovered after 2 minutes: Google" {
		t.Fatalf("fallback=%q", meta.FallbackText())
	}
	if meta.DowntimeLabel() != "2m 14s" {
		t.Fatalf("downtime=%q", meta.DowntimeLabel())
	}
	if meta.IncidentLabel() != "INC-ABCDEF12" {
		t.Fatalf("incident=%q", meta.IncidentLabel())
	}
	if meta.ResponseLabel() != "168ms" {
		t.Fatalf("response=%q", meta.ResponseLabel())
	}

	down := AlertMeta{Event: "DOWN", Name: "Test"}
	if down.Title() != "Outage Detected" {
		t.Fatalf("down title=%q", down.Title())
	}
	if down.FallbackText() != "Outage Detected: Test" {
		t.Fatalf("down fallback=%q", down.FallbackText())
	}
}

func TestFormatRecoveredAfter(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "Recovered after 1 minute"},
		{time.Minute + 5*time.Second, "Recovered after 1 minute"},
		{2*time.Minute + 14*time.Second, "Recovered after 2 minutes"},
		{59 * time.Minute, "Recovered after 59 minutes"},
		{time.Hour, "Recovered after 1 hour"},
		{90 * time.Minute, "Recovered after 2 hours"},
		{3 * time.Hour, "Recovered after 3 hours"},
	}
	for _, tc := range cases {
		if got := formatRecoveredAfter(tc.d); got != tc.want {
			t.Fatalf("formatRecoveredAfter(%v)=%q, want %q", tc.d, got, tc.want)
		}
	}

	recovery := AlertMeta{Event: "RECOVERY", Name: "Test"}
	if recovery.Title() != "Recovered" {
		t.Fatalf("recovery without duration title=%q", recovery.Title())
	}

	started := time.Date(2026, 9, 16, 12, 13, 0, 0, time.UTC)
	disk := AlertMeta{
		Event:          "RECOVERY",
		Name:           "host",
		EventAt:        started.Add(13 * time.Minute),
		StartedAt:      &started,
		RecoveredLabel: "Disk usage",
	}
	if disk.Title() != "Disk usage recovered after 13 minutes" {
		t.Fatalf("disk recovery title=%q", disk.Title())
	}
	if disk.FallbackText() != "Disk usage recovered after 13 minutes: host" {
		t.Fatalf("disk recovery fallback=%q", disk.FallbackText())
	}
}

func TestAlertMetaTimeoutLabel(t *testing.T) {
	meta := AlertMeta{Event: "DOWN", Message: "context deadline exceeded", ResponseMs: 10000}
	if meta.ResponseLabel() != "Timeout" {
		t.Fatalf("response=%q", meta.ResponseLabel())
	}
}

func TestBuildSlackPayload(t *testing.T) {
	meta := AlertMeta{
		Event:        "DOWN",
		Name:         "Google",
		URL:          "https://www.google.com/",
		Message:      "connection refused",
		DashboardURL: "http://localhost/monitors/1",
		ResponseMs:   176,
		IncidentID:   "deadbeef",
		EventAt:      time.Date(2026, 8, 11, 16, 13, 0, 0, time.UTC),
	}
	raw, err := buildSlackPayload(meta)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload["text"].(string), "Outage Detected: Google") {
		t.Fatalf("fallback text=%v", payload["text"])
	}
	atts := payload["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("attachments=%d", len(atts))
	}
	att := atts[0].(map[string]any)
	if att["color"] != "#E01E5A" {
		t.Fatalf("color=%v", att["color"])
	}
	blocks := att["blocks"].([]any)
	if len(blocks) < 3 {
		t.Fatalf("blocks=%d", len(blocks))
	}
}

func TestRenderAlertEmailDNSChangeTables(t *testing.T) {
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:        "DNS CHANGE",
		Name:         "AnusreeTravels",
		URL:          "https://anusreetravels.com",
		Message:      sampleDNSMsg,
		DashboardURL: "http://localhost/incidents/1",
		ResponseMs:   40,
		IncidentID:   "deadbeef",
		EventAt:      time.Now().UTC(),
	})
	for _, want := range []string{"DNS CHANGE", "Previous", "Current", "147.79.69.222", "147.79.69.29", "2a02:4780:1f:4ae:49b1:4f3b:b23f:aace"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in dns email html", want)
		}
	}
	if strings.Contains(html, "A record changed from 147.79.69.222, 93.127.173.8 to") {
		t.Fatal("email should not dump the semicolon sentence as the body")
	}
}

func TestBuildSlackPayloadDNSChange(t *testing.T) {
	raw, err := buildSlackPayload(AlertMeta{
		Event:   "DNS CHANGE",
		Name:    "AnusreeTravels",
		Message: sampleDNSMsg,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "*Previous*") || !strings.Contains(s, "*Current*") || !strings.Contains(s, "147.79.69.29") {
		t.Fatalf("slack payload missing dns tables: %s", s)
	}
	if strings.Contains(s, "_A record changed from") {
		t.Fatal("slack should not italicize the run-on sentence")
	}
}

func TestBuildSlackPayloadHostServices(t *testing.T) {
	raw, err := buildSlackPayload(AlertMeta{
		Event:   "HOST_SERVICE",
		Name:    "internal",
		Message: "Watched services not active: nginx (missing), sshd (failed)",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "*Services*") || !strings.Contains(s, "`nginx` missing") || !strings.Contains(s, "`sshd` failed") {
		t.Fatalf("slack payload missing service table: %s", s)
	}
	if strings.Contains(s, "_Watched services not active") {
		t.Fatal("slack should not italicize the run-on sentence")
	}
}

func TestAlertMetaHostDiskWarningVsCritical(t *testing.T) {
	warn := AlertMeta{
		Event:    "HOST_DISK",
		Message:  "Disk warning: 83.0% (warning 80.0 / critical 90.0) for 20 consecutive samples",
		Severity: "warning",
	}
	if warn.Title() != "HOST DISK WARNING" {
		t.Fatalf("warning title=%q", warn.Title())
	}
	if warn.StatusLabel() != "WARNING" {
		t.Fatalf("warning status=%q", warn.StatusLabel())
	}
	if warn.Color() != alertColorWarning {
		t.Fatalf("warning color=%q", warn.Color())
	}

	crit := AlertMeta{
		Event:    "HOST_DISK",
		Message:  "Disk critical: 93.0% (warning 80.0 / critical 90.0) for 20 consecutive samples",
		Severity: "critical",
	}
	if crit.Title() != "HOST DISK HIGH" {
		t.Fatalf("critical title=%q", crit.Title())
	}
	if crit.StatusLabel() != "HIGH" {
		t.Fatalf("critical status=%q", crit.StatusLabel())
	}
	if crit.Color() != alertColorDanger {
		t.Fatalf("critical color=%q", crit.Color())
	}

	inferred := AlertMeta{
		Event:   "HOST_DISK",
		Message: "Disk warning: 83.0% (warning 80.0 / critical 90.0) for 20 consecutive samples [/ 83%]",
	}
	if inferred.Title() != "HOST DISK WARNING" || inferred.StatusLabel() != "WARNING" {
		t.Fatalf("inferred warning title=%q status=%q", inferred.Title(), inferred.StatusLabel())
	}
}

func TestRenderAlertEmailHostDiskWarning(t *testing.T) {
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:        "HOST_DISK",
		Name:         "internal",
		URL:          "internal.buildsite.in",
		Message:      "Disk warning: 83.0% (warning 80.0 / critical 90.0) for 20 consecutive samples",
		DashboardURL: "http://localhost/hosts/1",
		IncidentID:   "b7c704c7",
		EventAt:      time.Date(2026, 9, 15, 5, 58, 0, 0, time.UTC),
		Severity:     "warning",
	})
	for _, want := range []string{"HOST DISK WARNING", "WARNING", alertColorWarning, "internal"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in email html", want)
		}
	}
	if strings.Contains(html, "HOST DISK HIGH") {
		t.Fatal("warning email should not use HOST DISK HIGH")
	}
}

func TestBuildSlackPayloadHostDiskWarning(t *testing.T) {
	raw, err := buildSlackPayload(AlertMeta{
		Event:    "HOST_DISK",
		Name:     "internal",
		Message:  "Disk warning: 83.0% (warning 80.0 / critical 90.0)",
		Severity: "warning",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "HOST DISK WARNING") || !strings.Contains(s, "WARNING") || !strings.Contains(s, ":large_yellow_circle:") {
		t.Fatalf("slack warning payload=%s", s)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	att := payload["attachments"].([]any)[0].(map[string]any)
	if att["color"] != alertColorWarning {
		t.Fatalf("color=%v", att["color"])
	}
}

func TestBuildSlackPayloadHostDiskRecovery(t *testing.T) {
	started := time.Date(2026, 9, 16, 12, 13, 0, 0, time.UTC)
	raw, err := buildSlackPayload(AlertMeta{
		Event:          "RECOVERY",
		Name:           "2hats-prod-internal",
		URL:            "server.buildsite.in",
		Message:        "Disk recovered: 64.1% (below 75.0%) [/ 64%, /tmp 64%, /var/tmp 64%]",
		DashboardURL:   "http://localhost/hosts/1",
		IncidentID:     "6c88c572",
		EventAt:        started.Add(13 * time.Minute),
		StartedAt:      &started,
		RecoveredLabel: "Disk usage",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"Disk usage recovered after 13 minutes: 2hats-prod-internal",
		"Disk usage recovered after 13 minutes",
		"Disk recovered: 64.1%",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in slack recovery payload: %s", want, s)
		}
	}
	if strings.Contains(s, "*Services*") {
		t.Fatal("disk recovery must not render a Slack services table")
	}
}

func TestRenderAlertEmail(t *testing.T) {
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:        "DOWN",
		Name:         "Google",
		URL:          "https://www.google.com/",
		Message:      "connection refused",
		DashboardURL: "http://localhost/monitors/1",
		ResponseMs:   176,
		IncidentID:   "deadbeef",
		EventAt:      time.Now().UTC(),
	})
	for _, want := range []string{"Outage Detected", "Google", "https://www.google.com/", "INC-DEADBEEF", "Open in Sentinel", "#E01E5A"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in email html", want)
		}
	}
}

func TestRenderAlertEmailRecovery(t *testing.T) {
	started := time.Date(2026, 9, 16, 9, 23, 0, 0, time.UTC)
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:        "RECOVERY",
		Name:         "Test",
		URL:          "https://www.google.com/",
		DashboardURL: "http://localhost/monitors/1",
		ResponseMs:   114,
		IncidentID:   "d0b87b35",
		EventAt:      started.Add(time.Minute + 5*time.Second),
		StartedAt:    &started,
	})
	for _, want := range []string{"Recovered after 1 minute", "Test", "1m 5s"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in recovery email html", want)
		}
	}
	if strings.Contains(html, "[Sentinel]") || strings.Contains(html, "MONITOR RECOVERED") {
		t.Fatal("recovery email still uses old copy")
	}
}

func TestRenderAlertEmailHostDiskRecovery(t *testing.T) {
	started := time.Date(2026, 9, 16, 12, 13, 0, 0, time.UTC)
	a := &Alerter{}
	html := a.renderAlertEmail(AlertMeta{
		Event:          "RECOVERY",
		Name:           "2hats-prod-internal",
		URL:            "server.buildsite.in",
		Message:        "Disk recovered: 64.1% (below 75.0%) [/ 64%, /tmp 64%, /var/tmp 64%]",
		DashboardURL:   "http://localhost/hosts/1",
		IncidentID:     "6c88c572",
		EventAt:        started.Add(13 * time.Minute),
		StartedAt:      &started,
		RecoveredLabel: "Disk usage",
	})
	for _, want := range []string{"Disk usage recovered after 13 minutes", "Disk recovered: 64.1%", "2hats-prod-internal"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in host disk recovery html", want)
		}
	}
	if strings.Contains(html, ">Service<") || strings.Contains(html, ">Services<") {
		t.Fatal("disk recovery must not render a services table")
	}
}
