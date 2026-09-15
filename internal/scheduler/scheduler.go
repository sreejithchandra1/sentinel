package scheduler

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/alerter"
	"github.com/sentinel-monitoring/sentinel/internal/checker"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

type prober interface {
	Probe(ctx context.Context, m *models.Monitor) *models.CheckResult
	ProbePerformance(ctx context.Context, t *models.PerformanceTarget) *models.PerformanceResult
}

type probeJob struct {
	monitor *models.Monitor
	target  *models.PerformanceTarget
}

type Scheduler struct {
	store     *store.Store
	checker   prober
	alerter   *alerter.Alerter
	workers   int
	retention int
	jobs      chan probeJob
	lastRun   map[string]time.Time
	inFlight  map[string]struct{}
	mu        sync.Mutex
}

func New(s *store.Store, c *checker.Checker, a *alerter.Alerter, workers, retentionDays int) *Scheduler {
	return newScheduler(s, c, a, workers, retentionDays)
}

func newScheduler(s *store.Store, c prober, a *alerter.Alerter, workers, retentionDays int) *Scheduler {
	if workers < 1 {
		workers = 1
	}
	queue := workers * 4
	if queue < 8 {
		queue = 8
	}
	return &Scheduler{
		store:     s,
		checker:   c,
		alerter:   a,
		workers:   workers,
		retention: retentionDays,
		jobs:      make(chan probeJob, queue),
		lastRun:   make(map[string]time.Time),
		inFlight:  make(map[string]struct{}),
	}
}

func (sch *Scheduler) Start(ctx context.Context) {
	for i := 0; i < sch.workers; i++ {
		go sch.worker(ctx)
	}

	ticker := time.NewTicker(5 * time.Second)
	pruneTicker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sch.enqueueMonitors(ctx)
			sch.enqueuePerformance(ctx)
		case <-pruneTicker.C:
			sch.prune()
		}
	}
}

func (sch *Scheduler) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-sch.jobs:
			sch.runJob(ctx, job)
		}
	}
}

func (sch *Scheduler) runJob(ctx context.Context, job probeJob) {
	switch {
	case job.monitor != nil:
		id := monitorKey(job.monitor.ID)
		sch.noteRun(id)
		defer sch.releaseRun(id)
		sch.runCheck(ctx, job.monitor)
	case job.target != nil:
		id := perfKey(job.target.ID)
		sch.noteRun(id)
		defer sch.releaseRun(id)
		sch.runPerformanceCheck(ctx, job.target)
	}
}

func (sch *Scheduler) enqueueMonitors(ctx context.Context) {
	monitors, err := sch.store.ListEnabledMonitors()
	if err != nil {
		log.Printf("scheduler: list monitors: %v", err)
		return
	}

	now := time.Now().UTC()
	for i := range monitors {
		m := monitors[i]
		inMaint, err := sch.store.IsInMaintenance(m.ID, now)
		if err == nil && inMaint {
			continue
		}
		key := monitorKey(m.ID)
		if !sch.claimRun(key, m.IntervalSeconds, m.LastCheckedAt) {
			continue
		}
		job := probeJob{monitor: &m}
		if !sch.submit(ctx, job) {
			sch.releaseRun(key)
		}
	}
}

func (sch *Scheduler) enqueuePerformance(ctx context.Context) {
	targets, err := sch.store.ListEnabledPerformanceTargets()
	if err != nil {
		log.Printf("scheduler: list performance targets: %v", err)
		return
	}

	for i := range targets {
		t := targets[i]
		key := perfKey(t.ID)
		if !sch.claimRun(key, t.IntervalSeconds, t.LastCheckedAt) {
			continue
		}
		job := probeJob{target: &t}
		if !sch.submit(ctx, job) {
			sch.releaseRun(key)
		}
	}
}

func (sch *Scheduler) submit(ctx context.Context, job probeJob) bool {
	select {
	case sch.jobs <- job:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func (sch *Scheduler) claimRun(id string, intervalSec int, lastChecked *time.Time) bool {
	if intervalSec < 1 {
		intervalSec = 60
	}
	interval := time.Duration(intervalSec) * time.Second

	sch.mu.Lock()
	defer sch.mu.Unlock()
	if _, busy := sch.inFlight[id]; busy {
		return false
	}
	last, ok := sch.lastRun[id]
	if !ok {
		if lastChecked != nil && !lastChecked.IsZero() {
			last = lastChecked.UTC()
			sch.lastRun[id] = last
		} else {
			sch.inFlight[id] = struct{}{}
			return true
		}
	}
	if time.Since(last) < interval {
		return false
	}
	sch.inFlight[id] = struct{}{}
	return true
}

func (sch *Scheduler) noteRun(id string) {
	sch.mu.Lock()
	sch.lastRun[id] = time.Now()
	sch.mu.Unlock()
}

func (sch *Scheduler) releaseRun(id string) {
	sch.mu.Lock()
	delete(sch.inFlight, id)
	sch.mu.Unlock()
}

func (sch *Scheduler) runCheck(ctx context.Context, m *models.Monitor) {
	result := sch.checker.Probe(ctx, m)
	result.Status = models.InvertMonitorStatus(m.Invert, result.Status)
	if m.Invert && result.Status == models.StatusDown && strings.TrimSpace(result.Error) == "" {
		result.Error = "invert is on and the host responded"
	}
	if err := sch.store.InsertCheckResult(result); err != nil {
		log.Printf("scheduler: save result: %v", err)
		return
	}
	if err := sch.alerter.HandleResult(m, result); err != nil {
		log.Printf("scheduler: alert: %v", err)
	}
	if err := sch.alerter.HandleExtras(m, result); err != nil {
		log.Printf("scheduler: extra alerts: %v", err)
	}
}

func (sch *Scheduler) runPerformanceCheck(ctx context.Context, t *models.PerformanceTarget) {
	result := sch.checker.ProbePerformance(ctx, t)
	if err := sch.store.InsertPerformanceResult(result); err != nil {
		log.Printf("scheduler: save performance result: %v", err)
		return
	}
	status := result.Status
	prevStatus := t.LastStatus
	consecutive := t.ConsecutiveSlow
	if status == models.StatusDegraded {
		consecutive++
	} else {
		consecutive = 0
	}
	if err := sch.store.UpdatePerformanceTargetAfterCheck(t.ID, status, consecutive, result.CheckedAt); err != nil {
		log.Printf("scheduler: update performance target: %v", err)
	}
	t.LastStatus = status
	t.ConsecutiveSlow = consecutive
	if err := sch.alerter.HandlePerformanceResult(t, result, prevStatus); err != nil {
		log.Printf("scheduler: performance alert: %v", err)
	}
}

func (sch *Scheduler) prune() {
	days := sch.retention
	if days < 30 {
		days = 30
	}
	before := time.Now().AddDate(0, 0, -days)
	n, err := sch.store.PruneOldResults(before)
	if err != nil {
		log.Printf("scheduler: prune: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: pruned %d old check results", n)
	}
	pn, err := sch.store.PruneOldPerformanceResults(before)
	if err != nil {
		log.Printf("scheduler: prune performance: %v", err)
	} else if pn > 0 {
		log.Printf("scheduler: pruned %d old performance results", pn)
	}
	en, err := sch.store.PruneOldEmailLog(before)
	if err != nil {
		log.Printf("scheduler: prune email log: %v", err)
	} else if en > 0 {
		log.Printf("scheduler: pruned %d old email log rows", en)
	}
}

func monitorKey(id string) string { return "m:" + id }
func perfKey(id string) string    { return "p:" + id }
