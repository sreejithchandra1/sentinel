package checker

import (
	"context"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func (c *Checker) ProbePerformance(ctx context.Context, t *models.PerformanceTarget) *models.PerformanceResult {
	m := &models.Monitor{
		ID:              t.ID,
		Type:            models.MonitorHTTP,
		Name:            t.Name,
		URL:             t.URL,
		Method:          t.Method,
		ExpectedStatus:  200,
		IntervalSeconds: t.IntervalSeconds,
		TimeoutMs:       t.TimeoutMs,
		SlowThresholdMs: t.SlowThresholdMs,
		FollowRedirects: t.FollowRedirects,
		Enabled:         true,
	}
	if m.Method == "" {
		m.Method = "GET"
	}

	cr := c.probeHTTP(ctx, m)
	return &models.PerformanceResult{
		TargetID:       t.ID,
		Status:         performanceStatus(cr, t.SlowThresholdMs),
		StatusCode:     cr.StatusCode,
		ResponseTimeMs: cr.ResponseTimeMs,
		DNSMs:          cr.DNSMs,
		TCPMs:          cr.TCPMs,
		TLSMs:          cr.TLSMs,
		TTFBMs:         cr.TTFBMs,
		Error:          cr.Error,
		CheckedAt:      cr.CheckedAt,
	}
}

// performanceStatus keeps reachability failures as down. Slow is only a completed
// response whose wall time exceeds the target threshold (including timeout-after-reach).
func performanceStatus(cr *models.CheckResult, slowThresholdMs int) models.MonitorStatus {
	if cr == nil || cr.Status == models.StatusDown {
		return models.StatusDown
	}
	if slowThresholdMs > 0 && cr.ResponseTimeMs > slowThresholdMs {
		return models.StatusDegraded
	}
	return models.StatusUp
}
