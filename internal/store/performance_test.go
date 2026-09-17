package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestListMonitorRowStats(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a := &models.Monitor{Name: "a", URL: "https://a.example", Enabled: true}
	b := &models.Monitor{Name: "b", URL: "https://b.example", Enabled: true}
	if err := st.CreateMonitor(a); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateMonitor(b); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	insert := func(monitorID string, i int, status models.MonitorStatus, rt int) {
		t.Helper()
		if err := st.InsertCheckResult(&models.CheckResult{
			MonitorID:      monitorID,
			Status:         status,
			ResponseTimeMs: rt,
			CheckedAt:      now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 26; i++ {
		status := models.StatusUp
		if i == 0 {
			status = models.StatusDown
		}
		insert(a.ID, i, status, 100+i)
	}
	insert(b.ID, 0, models.StatusUp, 50)
	insert(b.ID, 1, models.StatusDegraded, 80)

	got, err := st.ListMonitorRowStats([]string{a.ID, b.ID, "missing"}, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}

	full, err := st.GetMonitorStats(a.ID, since, now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	row := got[a.ID]
	if row.UptimePct != full.UptimePct {
		t.Fatalf("a uptime=%v, want %v", row.UptimePct, full.UptimePct)
	}
	if len(row.Points) != 24 {
		t.Fatalf("a points=%d, want 24", len(row.Points))
	}
	if row.Points[0] != 102 || row.Points[23] != 125 {
		t.Fatalf("a points=%v, want last 24 latencies 102..125", row.Points)
	}

	if got[b.ID].UptimePct != 100 {
		t.Fatalf("b uptime=%v, want 100", got[b.ID].UptimePct)
	}
	if len(got[b.ID].Points) != 2 || got[b.ID].Points[0] != 50 || got[b.ID].Points[1] != 80 {
		t.Fatalf("b points=%v", got[b.ID].Points)
	}
	if got["missing"].UptimePct != 0 || len(got["missing"].Points) != 0 {
		t.Fatalf("missing=%+v", got["missing"])
	}
}

func TestKeepSpikePreservesMaxInBucket(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	buckets := map[int64]models.StatsPoint{}
	keepSpike(buckets, models.StatsPoint{Timestamp: now, ResponseTimeMs: 10}, time.Minute)
	keepSpike(buckets, models.StatsPoint{Timestamp: now.Add(10 * time.Second), ResponseTimeMs: 9999}, time.Minute)
	keepSpike(buckets, models.StatsPoint{Timestamp: now.Add(20 * time.Second), ResponseTimeMs: 5}, time.Minute)
	pts := sortedBuckets(buckets)
	if len(pts) != 1 || pts[0].ResponseTimeMs != 9999 {
		t.Fatalf("got %#v", pts)
	}
}

func TestChartBucketDuration(t *testing.T) {
	cases := []struct {
		span time.Duration
		want time.Duration
	}{
		{15 * time.Minute, time.Minute},
		{1 * time.Hour, 2 * time.Minute},
		{6 * time.Hour, 5 * time.Minute},
		{24 * time.Hour, 15 * time.Minute},
		{7 * 24 * time.Hour, time.Hour},
		{30 * 24 * time.Hour, 6 * time.Hour},
		{90 * 24 * time.Hour, 24 * time.Hour},
	}
	for _, tc := range cases {
		if got := chartBucketDuration(tc.span); got != tc.want {
			t.Fatalf("span %v: got %v want %v", tc.span, got, tc.want)
		}
	}
}

func TestGetMonitorStatsCapsReturnedPoints(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{Name: "cap", URL: "https://cap.example", Enabled: true}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 500; i++ {
		if err := st.InsertCheckResult(&models.CheckResult{
			MonitorID:      m.ID,
			Status:         models.StatusUp,
			ResponseTimeMs: i,
			CheckedAt:      now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	from := now
	to := now.Add(500 * time.Minute)
	stats, err := st.GetMonitorStats(m.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Points) == 0 || len(stats.Points) > 500 {
		t.Fatalf("points=%d, want auto-bucketed subset", len(stats.Points))
	}
	if stats.UptimePct != 100 {
		t.Fatalf("uptime=%v, want 100 (full series, not downsampled)", stats.UptimePct)
	}
	found := false
	for _, p := range stats.Points {
		if p.ResponseTimeMs == 499 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected max spike 499 to be kept")
	}
}

func TestGetPerformanceTargetStatsExcludesFailed(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := &models.PerformanceTarget{
		Name:            "lat",
		URL:             "https://lat.example",
		Enabled:         true,
		SlowThresholdMs: 3000,
	}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	insert := func(status models.MonitorStatus, rt int, i int) {
		t.Helper()
		if err := st.InsertPerformanceResult(&models.PerformanceResult{
			TargetID:       target.ID,
			Status:         status,
			ResponseTimeMs: rt,
			CheckedAt:      now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	insert(models.StatusUp, 100, 0)
	insert(models.StatusUp, 120, 1)
	insert(models.StatusDown, 10000, 2)

	stats, err := st.GetPerformanceTargetStats(target.ID, now.Add(-time.Minute), now.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Points) != 3 {
		t.Fatalf("points=%d want 3 (failures stay on the chart)", len(stats.Points))
	}
	if stats.Performance.MaxMs != 120 {
		t.Fatalf("max=%d want 120 (failed timeout excluded from percentiles)", stats.Performance.MaxMs)
	}
	if stats.AvgResponse != 110 {
		t.Fatalf("avg=%d want 110", stats.AvgResponse)
	}

	pct, total, slow, err := st.GetPerformanceSlowStats(target.ID, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || slow != 0 {
		t.Fatalf("slow stats total=%d slow=%d pct=%v want total=2 slow=0", total, slow, pct)
	}
}

func TestSlowIncidentCanReferencePerformanceTarget(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := &models.PerformanceTarget{Name: "p", URL: "https://p.example", Enabled: true}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}
	inc := &models.Incident{MonitorID: target.ID, Type: models.IncidentSlow, Message: "slow", StartedAt: time.Now().UTC()}
	if err := st.CreateIncident(inc); err != nil {
		t.Fatal(err)
	}
	open, err := st.GetOpenIncident(target.ID, models.IncidentSlow)
	if err != nil {
		t.Fatal(err)
	}
	if open == nil {
		t.Fatal("expected open slow incident for performance target")
	}
}

func TestFillPerformanceHTTPAuthFromSiblingMonitor(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:         "authed",
		URL:          "https://secret.example/dashboard",
		HTTPUsername: "web-admin",
		HTTPPassword: "s3cret",
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	target := &models.PerformanceTarget{
		Name:         "authed",
		URL:          "https://secret.example/dashboard",
		HTTPUsername: "web-admin",
	}
	st.FillPerformanceHTTPAuth(target)
	if target.HTTPPassword != "s3cret" {
		t.Fatalf("password=%q", target.HTTPPassword)
	}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPerformanceTarget(target.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.HTTPUsername != "web-admin" || got.HTTPPassword != "s3cret" {
		t.Fatalf("stored user=%q pass=%q", got.HTTPUsername, got.HTTPPassword)
	}
}

func TestMigrateV22BackfillsPerformanceHTTPAuth(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:         "site",
		URL:          "https://auth.example/",
		HTTPUsername: "alice",
		HTTPPassword: "p@ss",
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	target := &models.PerformanceTarget{Name: "site", URL: "https://auth.example/"}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}
	if err := st.migrateV22(); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetPerformanceTarget(target.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.HTTPUsername != "alice" || got.HTTPPassword != "p@ss" {
		t.Fatalf("backfill user=%q pass=%q", got.HTTPUsername, got.HTTPPassword)
	}
}
