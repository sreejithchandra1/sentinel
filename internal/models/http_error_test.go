package models

import "testing"

func TestStripHTTPErrorSourceSuffix(t *testing.T) {
	got := StripHTTPErrorSourceSuffix("expected status 200, got 404 (nginx error page)")
	if got != "expected status 200, got 404" {
		t.Fatalf("got %q", got)
	}
	got = StripHTTPErrorSourceSuffix("expected status 200, got 503 (Shopware maintenance)")
	if got != "expected status 200, got 503" {
		t.Fatalf("got %q", got)
	}
	plain := "connection timed out (host unreachable)"
	if StripHTTPErrorSourceSuffix(plain) != plain {
		t.Fatalf("should leave timeout message alone")
	}
}
