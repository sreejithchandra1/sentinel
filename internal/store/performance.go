package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const monitorSparklinePoints = 24
const maxChartPoints = 400

// downsamplePoints keeps at most max points, preserving spikes by taking the
// highest latency sample in each bucket. Percentiles are computed on the full set.
func downsamplePoints(points []models.StatsPoint, max int) []models.StatsPoint {
	n := len(points)
	if max <= 0 || n <= max {
		return points
	}
	out := make([]models.StatsPoint, 0, max)
	for i := 0; i < max; i++ {
		start := i * n / max
		end := (i + 1) * n / max
		if end <= start {
			continue
		}
		best := points[start]
		for _, p := range points[start:end] {
			if p.ResponseTimeMs > best.ResponseTimeMs {
				best = p
			}
		}
		out = append(out, best)
	}
	return out
}

func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func computePerformance(times []int, slowCount int, total int) models.PerformanceMetrics {
	if len(times) == 0 {
		return models.PerformanceMetrics{}
	}

	sorted := append([]int(nil), times...)
	sort.Ints(sorted)

	totalRT := 0
	for _, t := range times {
		totalRT += t
	}

	m := models.PerformanceMetrics{
		AvgMs:     totalRT / len(times),
		MinMs:     sorted[0],
		MaxMs:     sorted[len(sorted)-1],
		P50Ms:     percentile(sorted, 0.50),
		P95Ms:     percentile(sorted, 0.95),
		P99Ms:     percentile(sorted, 0.99),
		SlowCount: slowCount,
	}
	if total > 0 {
		m.DegradedPct = float64(slowCount) / float64(total) * 100
	}
	return m
}

func targetHealth(hasData bool, p95, threshold int) string {
	if !hasData {
		return "collecting"
	}
	// Slow only when P95 breaches the configured SLA — not merely because any
	// historical check was tagged degraded (stale/timeout mislabels).
	if threshold > 0 && p95 >= threshold {
		return "warning"
	}
	return "good"
}

func fleetBucketDuration(since time.Time) time.Duration {
	span := time.Since(since)
	switch {
	case span > 14*24*time.Hour:
		return 24 * time.Hour
	case span > 48*time.Hour:
		return 6 * time.Hour
	default:
		return time.Hour
	}
}

func (s *Store) GetMonitorStats(monitorID string, since time.Time) (*models.MonitorStats, error) {
	t0 := time.Now()
	rows, err := s.db.Query(`
		SELECT checked_at, response_time_ms, status, dns_ms, tcp_ms, tls_ms, ttfb_ms
		FROM check_results
		WHERE monitor_id = ? AND checked_at >= ?
		ORDER BY checked_at ASC`,
		monitorID, formatTime(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &models.MonitorStats{MonitorID: monitorID}
	var times []int
	var totalRT, upCount, slowCount, total int

	for rows.Next() {
		var checkedAt string
		var rt int
		var status string
		var dnsMs, tcpMs, tlsMs, ttfbMs sql.NullInt64
		if err := rows.Scan(&checkedAt, &rt, &status, &dnsMs, &tcpMs, &tlsMs, &ttfbMs); err != nil {
			return nil, err
		}

		t, _ := parseTime(checkedAt)
		stats.Points = append(stats.Points, models.StatsPoint{
			Timestamp:      t,
			ResponseTimeMs: rt,
			Status:         status,
			DNSMs:          nullableInt(dnsMs),
			TCPMs:          nullableInt(tcpMs),
			TLSMs:          nullableInt(tlsMs),
			TTFBMs:         nullableInt(ttfbMs),
		})
		times = append(times, rt)
		totalRT += rt
		total++
		if status == string(models.StatusDegraded) {
			slowCount++
		}
		if status == string(models.StatusUp) || status == string(models.StatusDegraded) {
			upCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if total > 0 {
		stats.AvgResponse = totalRT / total
		stats.UptimePct = float64(upCount) / float64(total) * 100
	}
	stats.Performance = computePerformance(times, slowCount, total)
	rawPoints := len(stats.Points)
	stats.Points = downsamplePoints(stats.Points, maxChartPoints)
	// #region agent log
	if f, err := os.OpenFile("/Users/Sreejith/monitoring-tool/.cursor/debug-f7d7ab.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		_ = json.NewEncoder(f).Encode(map[string]any{
			"sessionId": "f7d7ab", "runId": "post-fix", "hypothesisId": "E",
			"location": "performance.go:GetMonitorStats", "message": "stats query complete",
			"data": map[string]any{
				"monitorID": monitorID, "pointsRaw": rawPoints, "pointsOut": len(stats.Points),
				"ms": time.Since(t0).Milliseconds(),
			},
			"timestamp": time.Now().UnixMilli(),
		})
		_ = f.Close()
	}
	// #endregion
	return stats, nil
}

func (s *Store) ListMonitorRowStats(monitorIDs []string, since time.Time) (map[string]models.MonitorRowStats, error) {
	out := make(map[string]models.MonitorRowStats, len(monitorIDs))
	for _, id := range monitorIDs {
		out[id] = models.MonitorRowStats{Points: []int{}}
	}
	if len(monitorIDs) == 0 {
		return out, nil
	}

	placeholders := sqlPlaceholders(len(monitorIDs))
	args := make([]any, 0, len(monitorIDs)+1)
	for _, id := range monitorIDs {
		args = append(args, id)
	}
	args = append(args, formatTime(since))

	uptimeRows, err := s.db.Query(`
		SELECT monitor_id,
			100.0 * SUM(CASE WHEN status IN ('up', 'degraded') THEN 1 ELSE 0 END) / COUNT(*)
		FROM check_results
		WHERE monitor_id IN (`+placeholders+`) AND checked_at >= ?
		GROUP BY monitor_id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	for uptimeRows.Next() {
		var id string
		var pct float64
		if err := uptimeRows.Scan(&id, &pct); err != nil {
			uptimeRows.Close()
			return nil, err
		}
		row := out[id]
		row.UptimePct = pct
		out[id] = row
	}
	if err := uptimeRows.Err(); err != nil {
		uptimeRows.Close()
		return nil, err
	}
	if err := uptimeRows.Close(); err != nil {
		return nil, err
	}

	sparkArgs := make([]any, len(args)+1)
	copy(sparkArgs, args)
	sparkArgs[len(args)] = monitorSparklinePoints
	sparkRows, err := s.db.Query(`
		SELECT monitor_id, response_time_ms FROM (
			SELECT monitor_id, response_time_ms, checked_at, id,
				ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY checked_at DESC, id DESC) AS rn
			FROM check_results
			WHERE monitor_id IN (`+placeholders+`) AND checked_at >= ?
		) ranked
		WHERE rn <= ?
		ORDER BY monitor_id, checked_at ASC, id ASC`,
		sparkArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer sparkRows.Close()

	points := make(map[string][]int, len(monitorIDs))
	for sparkRows.Next() {
		var id string
		var rt int
		if err := sparkRows.Scan(&id, &rt); err != nil {
			return nil, err
		}
		points[id] = append(points[id], rt)
	}
	if err := sparkRows.Err(); err != nil {
		return nil, err
	}
	for id, pts := range points {
		row := out[id]
		row.Points = pts
		out[id] = row
	}
	return out, nil
}

func (s *Store) GetPerformanceTargetStats(targetID string, since time.Time) (*models.PerformanceStats, error) {
	var threshold int
	_ = s.db.QueryRow(`SELECT slow_threshold_ms FROM performance_targets WHERE id = ?`, targetID).Scan(&threshold)

	rows, err := s.db.Query(`
		SELECT checked_at, response_time_ms, status, dns_ms, tcp_ms, tls_ms, ttfb_ms
		FROM performance_results
		WHERE target_id = ? AND checked_at >= ?
		ORDER BY checked_at ASC`,
		targetID, formatTime(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &models.PerformanceStats{TargetID: targetID}
	var times []int
	var totalRT, slowCount, total int

	for rows.Next() {
		var checkedAt string
		var rt int
		var status string
		var dnsMs, tcpMs, tlsMs, ttfbMs sql.NullInt64
		if err := rows.Scan(&checkedAt, &rt, &status, &dnsMs, &tcpMs, &tlsMs, &ttfbMs); err != nil {
			return nil, err
		}

		t, _ := parseTime(checkedAt)
		stats.Points = append(stats.Points, models.StatsPoint{
			Timestamp:      t,
			ResponseTimeMs: rt,
			Status:         status,
			DNSMs:          nullableInt(dnsMs),
			TCPMs:          nullableInt(tcpMs),
			TLSMs:          nullableInt(tlsMs),
			TTFBMs:         nullableInt(ttfbMs),
		})
		times = append(times, rt)
		totalRT += rt
		total++
		if threshold > 0 && rt > threshold {
			slowCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if total > 0 {
		stats.AvgResponse = totalRT / total
	}
	stats.Performance = computePerformance(times, slowCount, total)
	return stats, nil
}

func (s *Store) GetPerformanceSlowStats(targetID string, since time.Time) (slowPct float64, total, slow int, err error) {
	var threshold int
	_ = s.db.QueryRow(`SELECT slow_threshold_ms FROM performance_targets WHERE id = ?`, targetID).Scan(&threshold)

	var slowSum sql.NullInt64
	err = s.db.QueryRow(`
		SELECT COUNT(*),
			SUM(CASE WHEN ? > 0 AND response_time_ms > ? THEN 1 ELSE 0 END)
		FROM performance_results
		WHERE target_id = ? AND checked_at >= ?`,
		threshold, threshold, targetID, formatTime(since),
	).Scan(&total, &slowSum)
	if err != nil {
		return 0, 0, 0, err
	}
	if slowSum.Valid {
		slow = int(slowSum.Int64)
	}
	if total == 0 {
		return 0, 0, 0, nil
	}
	slowPct = float64(slow) / float64(total) * 100
	return slowPct, total, slow, nil
}

func (s *Store) GetFleetPerformance(since time.Time) (*models.FleetPerformance, error) {
	return s.GetFleetPerformanceScoped(since, "")
}

func (s *Store) GetFleetPerformanceByTenant(since time.Time, tenantID string) (*models.FleetPerformance, error) {
	if tenantID == "" {
		return &models.FleetPerformance{Monitors: []models.MonitorPerformance{}, Timeline: []models.FleetTimelinePoint{}}, nil
	}
	return s.GetFleetPerformanceScoped(since, tenantID)
}

func (s *Store) GetFleetPerformanceScoped(since time.Time, tenantID string) (*models.FleetPerformance, error) {
	type targetAgg struct {
		meta  models.MonitorPerformance
		times []int
		slow  int
	}

	targetsByID := map[string]*targetAgg{}
	q := `
		SELECT id, name, url, last_status, slow_threshold_ms
		FROM performance_targets WHERE enabled = 1`
	var args []any
	if tenantID != "" {
		q += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}
	q += ` ORDER BY created_at DESC`
	targetRows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer targetRows.Close()

	for targetRows.Next() {
		var id, name, url, lastStatus string
		var slowThreshold int
		if err := targetRows.Scan(&id, &name, &url, &lastStatus, &slowThreshold); err != nil {
			return nil, err
		}
		if lastStatus == string(models.StatusDown) {
			lastStatus = string(models.StatusDegraded)
		}
		targetsByID[id] = &targetAgg{meta: models.MonitorPerformance{
			MonitorID: id, MonitorName: name, Type: "http", URL: url,
			Status: lastStatus, SlowThresholdMs: slowThreshold,
		}}
	}
	if err := targetRows.Err(); err != nil {
		return nil, err
	}

	bucketDur := fleetBucketDuration(since)
	type bucketAgg struct {
		totalRT   int
		count     int
		slowCount int
		start     time.Time
	}
	buckets := map[int64]*bucketAgg{}
	var fleetTimes []int
	var totalChecks, totalSlow, totalRT int

	resultRows, err := s.db.Query(`
		SELECT target_id, checked_at, response_time_ms, status
		FROM performance_results
		WHERE checked_at >= ?
		ORDER BY checked_at ASC`, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer resultRows.Close()

	for resultRows.Next() {
		var targetID, checkedAt string
		var rt int
		var status string
		if err := resultRows.Scan(&targetID, &checkedAt, &rt, &status); err != nil {
			return nil, err
		}
		_ = status
		t, err := parseTime(checkedAt)
		if err != nil {
			continue
		}

		a, ok := targetsByID[targetID]
		if !ok {
			continue
		}
		a.times = append(a.times, rt)
		a.meta.CheckCount++
		// Count SLA breaches by latency, not by stored status (avoids stale degraded rows).
		breached := a.meta.SlowThresholdMs > 0 && rt > a.meta.SlowThresholdMs
		if breached {
			a.slow++
		}

		fleetTimes = append(fleetTimes, rt)
		totalChecks++
		totalRT += rt
		if breached {
			totalSlow++
		}

		bucketStart := t.Truncate(bucketDur)
		key := bucketStart.Unix()
		b, ok := buckets[key]
		if !ok {
			b = &bucketAgg{start: bucketStart}
			buckets[key] = b
		}
		b.totalRT += rt
		b.count++
		if breached {
			b.slowCount++
		}
	}
	if err := resultRows.Err(); err != nil {
		return nil, err
	}

	fleet := &models.FleetPerformance{
		MonitorCount: len(targetsByID),
		TotalChecks:  totalChecks,
		SlowChecks:   totalSlow,
	}
	if totalChecks > 0 {
		fleet.AvgMs = totalRT / totalChecks
	}
	fleetPerf := computePerformance(fleetTimes, totalSlow, totalChecks)
	fleet.P95Ms = fleetPerf.P95Ms

	timeline := make([]models.FleetTimelinePoint, 0, len(buckets))
	for _, b := range buckets {
		avg := 0
		if b.count > 0 {
			avg = b.totalRT / b.count
		}
		timeline = append(timeline, models.FleetTimelinePoint{
			Timestamp:  b.start,
			AvgMs:      avg,
			CheckCount: b.count,
			SlowCount:  b.slowCount,
		})
	}
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})
	fleet.Timeline = timeline

	services := make([]models.MonitorPerformance, 0, len(targetsByID))
	for _, a := range targetsByID {
		hasData := len(a.times) > 0
		if hasData {
			perf := computePerformance(a.times, a.slow, a.meta.CheckCount)
			a.meta.AvgMs = perf.AvgMs
			a.meta.P95Ms = perf.P95Ms
			a.meta.MaxMs = perf.MaxMs
			a.meta.SlowCount = a.slow
		}
		a.meta.HasData = hasData
		a.meta.Health = targetHealth(hasData, a.meta.P95Ms, a.meta.SlowThresholdMs)

		switch a.meta.Health {
		case "good":
			fleet.HealthyCount++
		case "warning":
			fleet.WarningCount++
		case "collecting":
			fleet.CollectingCount++
		}
		services = append(services, a.meta)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].HasData != services[j].HasData {
			return services[i].HasData
		}
		return services[i].P95Ms > services[j].P95Ms
	})
	fleet.Monitors = services
	return fleet, nil
}
