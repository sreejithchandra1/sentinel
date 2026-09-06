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
	if meta.Title() != "MONITOR RECOVERED" {
		t.Fatalf("title=%q", meta.Title())
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
	if !strings.Contains(payload["text"].(string), "DOWN: Google") {
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
	for _, want := range []string{"MONITOR DOWN", "Google", "https://www.google.com/", "INC-DEADBEEF", "Open in Sentinel", "#E01E5A"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in email html", want)
		}
	}
}
