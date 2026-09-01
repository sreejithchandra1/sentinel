package checker

import (
	"context"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestPerformanceStatus(t *testing.T) {
	up := &models.CheckResult{Status: models.StatusUp, ResponseTimeMs: 200}
	if got := performanceStatus(up, 3000); got != models.StatusUp {
		t.Fatalf("fast up: %s", got)
	}
	slow := &models.CheckResult{Status: models.StatusUp, ResponseTimeMs: 4000}
	if got := performanceStatus(slow, 3000); got != models.StatusDegraded {
		t.Fatalf("slow up: %s", got)
	}
	timeoutAfterReach := &models.CheckResult{Status: models.StatusUp, ResponseTimeMs: 10000}
	if got := performanceStatus(timeoutAfterReach, 3000); got != models.StatusDegraded {
		t.Fatalf("timeout-after-reach: %s", got)
	}
	fail := &models.CheckResult{Status: models.StatusDown, ResponseTimeMs: 10000, Error: "lookup failed"}
	if got := performanceStatus(fail, 3000); got != models.StatusDown {
		t.Fatalf("down stays down: %s", got)
	}
	if got := performanceStatus(nil, 3000); got != models.StatusDown {
		t.Fatalf("nil: %s", got)
	}
}

func TestProbePerformanceBlockedHostIsDownNotSlow(t *testing.T) {
	c := New(nil)
	got := c.ProbePerformance(context.Background(), &models.PerformanceTarget{
		Name:            "loopback",
		URL:             "http://127.0.0.1/",
		TimeoutMs:       2000,
		SlowThresholdMs: 1,
	})
	if got.Status != models.StatusDown {
		t.Fatalf("status=%s want down (not degraded)", got.Status)
	}
}
