package alerter

import (
	"strings"
	"testing"
)

func TestWebhookMatchesEvent(t *testing.T) {
	if !webhookMatchesEvent([]string{"all"}, "DOWN") {
		t.Fatal("all should match")
	}
	if !webhookMatchesEvent([]string{"DOWN", "RECOVERY"}, "down") {
		t.Fatal("case insensitive match")
	}
	if webhookMatchesEvent([]string{"RECOVERY"}, "DOWN") {
		t.Fatal("should not match")
	}
	if !webhookMatchesEvent(nil, "DOWN") {
		t.Fatal("empty events means all")
	}
}

func TestFormatSlackFallbackStillReadable(t *testing.T) {
	meta := AlertMeta{Event: "DOWN", Name: "Example", URL: "https://example.com", Message: "boom", ResponseMs: 12}
	if !strings.Contains(meta.FallbackText(), "Outage Detected: Example") {
		t.Fatalf("fallback=%q", meta.FallbackText())
	}
}
