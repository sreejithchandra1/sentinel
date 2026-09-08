package alerter

import (
	"fmt"
	"log"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func hostAsMonitor(h *models.Host) *models.Monitor {
	return &models.Monitor{
		ID:             h.ID,
		Name:           h.DisplayName(),
		URL:            h.Hostname,
		TenantID:       h.TenantID,
		AlertEmails:    h.AlertEmails,
		NotifyEmail:    h.NotifyEmail,
		NotifySlack:    h.NotifySlack,
		NotifyWebhooks: h.NotifyWebhooks,
		Enabled:        h.Enabled,
	}
}

func hostNotify(a *Alerter, h *models.Host, meta AlertMeta) error {
	m := hostAsMonitor(h)
	if meta.DashboardURL == "" {
		meta.DashboardURL = a.liveDashboardURL() + "/hosts/" + h.ID
	}
	return a.notifyMonitorAlert(m, meta)
}

func hostOfflineThreshold(h *models.Host) int {
	if h.AlertAfterFailures < 1 {
		return defaultFlapThreshold
	}
	return h.AlertAfterFailures
}

func (a *Alerter) HandleHostOfflineCheck(h *models.Host, now time.Time) error {
	if h == nil || !h.Enabled || h.LastSeenAt == nil {
		return nil
	}
	if now.Sub(*h.LastSeenAt) <= models.HostGrace(h) {
		return nil
	}

	threshold := hostOfflineThreshold(h)
	misses := h.ConsecutiveMisses + 1
	status := h.Status
	if misses >= threshold {
		status = models.HostOffline
	}
	if err := a.store.UpdateHostOfflineState(h.ID, status, misses, now); err != nil {
		return err
	}
	h.ConsecutiveMisses = misses
	h.Status = status
	if misses < threshold {
		log.Printf("alerter: host offline pending for %s: %d/%d missed intervals", h.DisplayName(), misses, threshold)
		return nil
	}

	open, err := a.store.GetOpenIncident(h.ID, models.IncidentHostOffline)
	if err != nil {
		return err
	}
	if open != nil {
		return nil
	}
	msg := fmt.Sprintf("Host has not reported in %s", models.HostGrace(h))
	inc := &models.Incident{
		MonitorID: h.ID,
		Type:      models.IncidentHostOffline,
		Message:   msg,
		StartedAt: now,
	}
	if err := a.store.CreateIncident(inc); err != nil {
		return err
	}
	return hostNotify(a, h, AlertMeta{
		Event:      "HOST_OFFLINE",
		Message:    msg,
		IncidentID: inc.ID,
		EventAt:    now,
	})
}

func (a *Alerter) HandleHostIngestOnline(h *models.Host, now time.Time) error {
	open, err := a.store.GetOpenIncident(h.ID, models.IncidentHostOffline)
	if err != nil {
		return err
	}
	if open == nil {
		return nil
	}
	started := open.StartedAt
	_ = a.store.ResolveOpenIncidents(h.ID, models.IncidentHostOffline, now)
	return hostNotify(a, h, AlertMeta{
		Event:      "RECOVERY",
		Message:    "Host agent is reporting again",
		IncidentID: open.ID,
		EventAt:    now,
		StartedAt:  &started,
	})
}

func (a *Alerter) HandleHostSample(h *models.Host, sample *models.HostSample) error {
	if h == nil || sample == nil || !h.Enabled {
		return nil
	}
	var first error
	setErr := func(err error) {
		if first == nil && err != nil {
			first = err
		}
	}
	if h.CollectCPU && h.AlertCPUEnabled && sample.CPUPercent != nil {
		setErr(a.evalHostGauge(h, sample, "cpu", *sample.CPUPercent, h.AlertCPUThreshold, h.AlertCPUThreshold-5, h.AlertCPUAfter, models.IncidentHostCPU, "HOST_CPU", "CPU"))
	}
	if h.CollectMemory && h.AlertMemoryEnabled && sample.MemPercent != nil {
		setErr(a.evalHostGauge(h, sample, "memory", *sample.MemPercent, h.AlertMemoryThreshold, h.AlertMemoryThreshold-5, h.AlertMemoryAfter, models.IncidentHostMemory, "HOST_MEMORY", "Memory"))
	}
	if h.CollectDisk && h.AlertDiskEnabled && sample.DiskPercent != nil {
		setErr(a.evalHostGauge(h, sample, "disk", *sample.DiskPercent, h.AlertDiskThreshold, h.AlertDiskThreshold-5, h.AlertDiskAfter, models.IncidentHostDisk, "HOST_DISK", "Disk"))
	}
	if h.CollectLoad && h.AlertLoadEnabled && sample.Load1 != nil {
		thresh := models.HostLoadThreshold(h, sample.NumCPU)
		setErr(a.evalHostGauge(h, sample, "load", *sample.Load1, thresh, thresh*0.8, h.AlertLoadAfter, models.IncidentHostLoad, "HOST_LOAD", "Load"))
	}
	return first
}

func (a *Alerter) evalHostGauge(
	h *models.Host,
	sample *models.HostSample,
	metric string,
	value, threshold, recoverBelow float64,
	after int,
	incType models.IncidentType,
	event, label string,
) error {
	if after < 1 {
		after = models.DefaultHostCPUAfter
	}
	state, err := a.store.GetHostAlertState(h.ID, metric)
	if err != nil {
		return err
	}
	open, err := a.store.GetOpenIncident(h.ID, incType)
	if err != nil {
		return err
	}

	switch {
	case value >= threshold:
		state.ConsecutiveHigh++
		state.ConsecutiveOK = 0
		if err := a.store.UpsertHostAlertState(state); err != nil {
			return err
		}
		if open != nil {
			return nil
		}
		if state.ConsecutiveHigh < after {
			log.Printf("alerter: host %s %s alert pending: %d/%d (value=%.1f threshold=%.1f)", h.DisplayName(), metric, state.ConsecutiveHigh, after, value, threshold)
			return nil
		}
		msg := fmt.Sprintf("%s %.1f is above threshold %.1f for %d consecutive samples", label, value, threshold, state.ConsecutiveHigh)
		inc := &models.Incident{
			MonitorID: h.ID,
			Type:      incType,
			Message:   msg,
			StartedAt: sample.CollectedAt,
		}
		if err := a.store.CreateIncident(inc); err != nil {
			return err
		}
		return hostNotify(a, h, AlertMeta{
			Event:      event,
			Message:    msg,
			IncidentID: inc.ID,
			EventAt:    sample.CollectedAt,
		})
	case value < recoverBelow:
		state.ConsecutiveOK++
		state.ConsecutiveHigh = 0
		if err := a.store.UpsertHostAlertState(state); err != nil {
			return err
		}
		if open == nil {
			return nil
		}
		if state.ConsecutiveOK < after {
			log.Printf("alerter: host %s %s recovery pending: %d/%d", h.DisplayName(), metric, state.ConsecutiveOK, after)
			return nil
		}
		started := open.StartedAt
		_ = a.store.ResolveOpenIncidents(h.ID, incType, sample.CollectedAt)
		return hostNotify(a, h, AlertMeta{
			Event:      "RECOVERY",
			Message:    fmt.Sprintf("%s recovered (%.1f below %.1f)", label, value, recoverBelow),
			IncidentID: open.ID,
			EventAt:    sample.CollectedAt,
			StartedAt:  &started,
		})
	default:
		state.ConsecutiveHigh = 0
		return a.store.UpsertHostAlertState(state)
	}
}
