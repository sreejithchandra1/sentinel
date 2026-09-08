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

type Scheduler struct {
	store       *store.Store
	checker     *checker.Checker
	alerter     *alerter.Alerter
	workers     int
	retention   int
	lastRun     map[string]time.Time
	perfLastRun map[string]time.Time
	mu          sync.Mutex
}

func New(s *store.Store, c *checker.Checker, a *alerter.Alerter, workers, retentionDays int) *Scheduler {
	return &Scheduler{
		store:       s,
		checker:     c,
		alerter:     a,
		workers:     workers,
		retention:   retentionDays,
		lastRun:     make(map[string]time.Time),
		perfLastRun: make(map[string]time.Time),
	}
}

func (sch *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	pruneTicker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sch.tick(ctx)
			sch.tickPerformance(ctx)
			sch.tickHosts()
		case <-pruneTicker.C:
			sch.prune()
		}
	}
}

func (sch *Scheduler) tick(ctx context.Context) {
	monitors, err := sch.store.ListEnabledMonitors()
	if err != nil {
		log.Printf("scheduler: list monitors: %v", err)
		return
	}

	sem := make(chan struct{}, sch.workers)
	var wg sync.WaitGroup

	for i := range monitors {
		m := monitors[i]
		inMaint, err := sch.store.IsInMaintenance(m.ID, time.Now().UTC())
		if err == nil && inMaint {
			continue
		}
		if !sch.shouldRun(m) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(mon models.Monitor) {
			defer wg.Done()
			defer func() { <-sem }()
			sch.runCheck(ctx, &mon)
		}(m)
	}
	wg.Wait()
}

func (sch *Scheduler) shouldRun(m models.Monitor) bool {
	sch.mu.Lock()
	defer sch.mu.Unlock()
	last, ok := sch.lastRun[m.ID]
	if !ok {
		sch.lastRun[m.ID] = time.Now()
		return true
	}
	if time.Since(last) >= time.Duration(m.IntervalSeconds)*time.Second {
		sch.lastRun[m.ID] = time.Now()
		return true
	}
	return false
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

func (sch *Scheduler) tickPerformance(ctx context.Context) {
	targets, err := sch.store.ListEnabledPerformanceTargets()
	if err != nil {
		log.Printf("scheduler: list performance targets: %v", err)
		return
	}

	sem := make(chan struct{}, sch.workers)
	var wg sync.WaitGroup

	for i := range targets {
		t := targets[i]
		if !sch.shouldRunPerf(t.ID, t.IntervalSeconds) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(target models.PerformanceTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			sch.runPerformanceCheck(ctx, &target)
		}(t)
	}
	wg.Wait()
}

func (sch *Scheduler) shouldRunPerf(id string, intervalSec int) bool {
	sch.mu.Lock()
	defer sch.mu.Unlock()
	last, ok := sch.perfLastRun[id]
	if !ok {
		sch.perfLastRun[id] = time.Now()
		return true
	}
	if time.Since(last) >= time.Duration(intervalSec)*time.Second {
		sch.perfLastRun[id] = time.Now()
		return true
	}
	return false
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
		// Up or failed: neither continues a slow streak. Failures are not recovery.
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

func (sch *Scheduler) tickHosts() {
	hosts, err := sch.store.ListEnabledHosts()
	if err != nil {
		log.Printf("scheduler: list hosts: %v", err)
		return
	}
	now := time.Now().UTC()
	for i := range hosts {
		h := hosts[i]
		if err := sch.alerter.HandleHostOfflineCheck(&h, now); err != nil {
			log.Printf("scheduler: host offline %s: %v", h.ID, err)
		}
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
	hn, err := sch.store.PruneOldHostSamples(before)
	if err != nil {
		log.Printf("scheduler: prune host samples: %v", err)
	} else if hn > 0 {
		log.Printf("scheduler: pruned %d old host samples", hn)
	}
}
