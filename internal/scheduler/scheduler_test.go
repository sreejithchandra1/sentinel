package scheduler

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/alerter"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

type stubProber struct {
	started chan struct{}
	block   chan struct{}
	delay   time.Duration
	probes  atomic.Int32
}

func (s *stubProber) Probe(_ context.Context, m *models.Monitor) *models.CheckResult {
	s.probes.Add(1)
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.block != nil {
		<-s.block
	}
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	code := 200
	return &models.CheckResult{
		MonitorID:  m.ID,
		Status:     models.StatusUp,
		StatusCode: &code,
		CheckedAt:  time.Now().UTC(),
	}
}

func (s *stubProber) ProbePerformance(_ context.Context, t *models.PerformanceTarget) *models.PerformanceResult {
	return &models.PerformanceResult{
		TargetID:  t.ID,
		Status:    models.StatusUp,
		CheckedAt: time.Now().UTC(),
	}
}

func testScheduler(t *testing.T, workers int, p prober) (*Scheduler, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	a := alerter.New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	return newScheduler(st, p, a, workers, 30), st
}

func createEnabledMonitor(t *testing.T, st *store.Store, name string) *models.Monitor {
	t.Helper()
	m := &models.Monitor{
		Name:            name,
		Type:            models.MonitorHTTP,
		URL:             "https://example.com/login",
		Method:          "GET",
		IntervalSeconds: 60,
		TimeoutMs:       5000,
		Enabled:         true,
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestClaimRunSkipsInFlightAndRespectsInterval(t *testing.T) {
	sch := newScheduler(nil, &stubProber{}, nil, 1, 30)
	id := monitorKey("mon-1")
	if !sch.claimRun(id, 1, nil) {
		t.Fatal("first claim should run")
	}
	if sch.claimRun(id, 1, nil) {
		t.Fatal("in-flight claim should skip")
	}
	sch.noteRun(id)
	sch.releaseRun(id)
	if sch.claimRun(id, 1, nil) {
		t.Fatal("should wait for interval after noteRun")
	}
	time.Sleep(1100 * time.Millisecond)
	if !sch.claimRun(id, 1, nil) {
		t.Fatal("should run after interval")
	}
}

func TestClaimRunUsesLastCheckedAt(t *testing.T) {
	sch := newScheduler(nil, &stubProber{}, nil, 1, 30)
	id := monitorKey("mon-2")
	recent := time.Now().Add(-10 * time.Second)
	if sch.claimRun(id, 60, &recent) {
		t.Fatal("recent last_checked_at should not be due")
	}
	stale := time.Now().Add(-2 * time.Minute)
	id2 := monitorKey("mon-3")
	if !sch.claimRun(id2, 60, &stale) {
		t.Fatal("stale last_checked_at should be due")
	}
}

func TestEnqueueDoesNotWaitForSlowProbe(t *testing.T) {
	prober := &stubProber{
		started: make(chan struct{}, 1),
		block:   make(chan struct{}),
	}
	sch, st := testScheduler(t, 1, prober)
	createEnabledMonitor(t, st, "slow")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.worker(ctx)

	start := time.Now()
	sch.enqueueMonitors(ctx)
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("enqueue blocked for %s", elapsed)
	}
	select {
	case <-prober.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	close(prober.block)
}

func TestInFlightPreventsOverlapWhileProbeRuns(t *testing.T) {
	prober := &stubProber{
		started: make(chan struct{}, 1),
		block:   make(chan struct{}),
	}
	sch, st := testScheduler(t, 2, prober)
	m := createEnabledMonitor(t, st, "once")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.worker(ctx)
	go sch.worker(ctx)

	sch.enqueueMonitors(ctx)
	select {
	case <-prober.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}

	sch.mu.Lock()
	sch.lastRun[monitorKey(m.ID)] = time.Now().Add(-time.Hour)
	sch.mu.Unlock()
	sch.enqueueMonitors(ctx)

	time.Sleep(50 * time.Millisecond)
	if got := prober.probes.Load(); got != 1 {
		t.Fatalf("probes=%d want 1 while first is in flight", got)
	}
	close(prober.block)
}

func TestDueMonitorRunsWhileAnotherIsSlow(t *testing.T) {
	var (
		mu      sync.Mutex
		started []string
		release = make(chan struct{})
	)
	prober := &countingProber{
		onProbe: func(id string) {
			mu.Lock()
			started = append(started, id)
			n := len(started)
			mu.Unlock()
			if n == 1 {
				<-release
			}
		},
	}
	sch, st := testScheduler(t, 2, prober)
	a := createEnabledMonitor(t, st, "a")
	b := createEnabledMonitor(t, st, "b")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.worker(ctx)
	go sch.worker(ctx)

	start := time.Now()
	sch.enqueueMonitors(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(started)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second monitor did not start while the first was still running")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if time.Since(start) > 300*time.Millisecond {
		t.Fatalf("second monitor waited on first probe: %s", time.Since(start))
	}
	close(release)

	seen := map[string]bool{}
	mu.Lock()
	for _, id := range started {
		seen[id] = true
	}
	mu.Unlock()
	if !seen[a.ID] || !seen[b.ID] {
		t.Fatalf("started=%v", started)
	}
}

type countingProber struct {
	onProbe func(id string)
}

func (c *countingProber) Probe(_ context.Context, m *models.Monitor) *models.CheckResult {
	if c.onProbe != nil {
		c.onProbe(m.ID)
	}
	code := 200
	return &models.CheckResult{
		MonitorID:  m.ID,
		Status:     models.StatusUp,
		StatusCode: &code,
		CheckedAt:  time.Now().UTC(),
	}
}

func (c *countingProber) ProbePerformance(_ context.Context, t *models.PerformanceTarget) *models.PerformanceResult {
	return &models.PerformanceResult{TargetID: t.ID, Status: models.StatusUp, CheckedAt: time.Now().UTC()}
}
