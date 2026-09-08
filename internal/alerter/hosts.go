package alerter

import (
	"fmt"
	"log"
	"strings"
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
		setErr(a.evalHostGauge(h, sample, "cpu", *sample.CPUPercent, h.AlertCPUWarning, h.AlertCPUThreshold, h.AlertCPUAfter, models.IncidentHostCPU, "HOST_CPU", "CPU", "%", ""))
	}
	if h.CollectMemory && h.AlertMemoryEnabled && sample.MemPercent != nil {
		setErr(a.evalHostGauge(h, sample, "memory", *sample.MemPercent, h.AlertMemoryWarning, h.AlertMemoryThreshold, h.AlertMemoryAfter, models.IncidentHostMemory, "HOST_MEMORY", "Memory", "%", ""))
	}
	if h.CollectSwap && h.AlertSwapEnabled && sample.SwapPercent != nil && *sample.SwapPercent > 0 {
		setErr(a.evalHostGauge(h, sample, "swap", *sample.SwapPercent, h.AlertSwapWarning, h.AlertSwapThreshold, h.AlertSwapAfter, models.IncidentHostSwap, "HOST_SWAP", "Swap", "%", ""))
	}
	if h.CollectIOWait && h.AlertIOWaitEnabled && sample.IOWaitPercent != nil {
		setErr(a.evalHostGauge(h, sample, "iowait", *sample.IOWaitPercent, h.AlertIOWaitWarning, h.AlertIOWaitThreshold, h.AlertIOWaitAfter, models.IncidentHostIOWait, "HOST_IOWAIT", "Disk I/O wait", "%", ""))
	}
	if h.CollectDisk && h.AlertDiskEnabled && len(sample.Disks) > 0 {
		worst, detail := worstDisk(sample.Disks, h.AlertDiskWarning)
		if worst != nil {
			setErr(a.evalHostGauge(h, sample, "disk", *worst, h.AlertDiskWarning, h.AlertDiskThreshold, h.AlertDiskAfter, models.IncidentHostDisk, "HOST_DISK", "Disk", "%", detail))
		}
	} else if h.CollectDisk && h.AlertDiskEnabled && sample.DiskPercent != nil {
		setErr(a.evalHostGauge(h, sample, "disk", *sample.DiskPercent, h.AlertDiskWarning, h.AlertDiskThreshold, h.AlertDiskAfter, models.IncidentHostDisk, "HOST_DISK", "Disk", "%", ""))
	}
	if h.CollectLoad && h.AlertLoadEnabled && sample.Load1 != nil {
		warn := models.HostLoadWarning(h, sample.NumCPU)
		crit := models.HostLoadCritical(h, sample.NumCPU)
		detail := fmt.Sprintf(" (%d CPU cores)", sample.NumCPU)
		if sample.NumCPU < 1 {
			detail = ""
		}
		setErr(a.evalHostGauge(h, sample, "load", *sample.Load1, warn, crit, h.AlertLoadAfter, models.IncidentHostLoad, "HOST_LOAD", "Load", "", detail))
	}
	if h.CollectSecurity && h.Security != nil {
		if h.AlertAuthEnabled {
			setErr(a.evalHostAuth(h, sample))
		}
		if h.AlertRootLoginEnabled {
			setErr(a.evalHostFlag(h, sample, "root_login", h.Security.RootLogins5m > 0,
				models.IncidentHostRootLogin, "HOST_ROOT_LOGIN", rootLoginMessage(h.Security)))
		}
	}
	if h.AlertRebootEnabled {
		setErr(a.evalHostFlag(h, sample, "reboot", h.RebootRequired,
			models.IncidentHostReboot, "HOST_REBOOT", "Kernel or package updates require a reboot"))
	}
	if h.CollectServices && h.AlertServiceEnabled && len(h.Services) > 0 {
		setErr(a.evalHostServices(h, sample))
	}
	return first
}

func worstDisk(disks []models.HostDisk, warning float64) (*float64, string) {
	var worst float64
	var over []string
	for _, d := range disks {
		if d.Percent > worst {
			worst = d.Percent
		}
		if d.Percent >= warning {
			over = append(over, fmt.Sprintf("%s %.0f%%", d.Mount, d.Percent))
		}
	}
	if worst <= 0 && len(disks) == 0 {
		return nil, ""
	}
	v := worst
	detail := ""
	if len(over) > 0 {
		detail = " [" + strings.Join(over, ", ") + "]"
	}
	return &v, detail
}

func rootLoginMessage(sec *models.HostSecurity) string {
	msg := fmt.Sprintf("Root login detected (%d in 5 minutes)", sec.RootLogins5m)
	if sec.LastRootLogin != "" {
		msg += " last at " + sec.LastRootLogin
	}
	return msg
}

func (a *Alerter) evalHostAuth(h *models.Host, sample *models.HostSample) error {
	sec := h.Security
	if sec == nil {
		return nil
	}
	total := sec.SSHFailed5m + sec.SudoFailed5m + sec.AuthFailed5m
	limit := h.AlertAuthThreshold
	if limit < 1 {
		limit = models.DefaultHostAuthFailLimit
	}
	msg := fmt.Sprintf("Authentication failures in 5 minutes: SSH %d, sudo %d, other %d (limit %d)",
		sec.SSHFailed5m, sec.SudoFailed5m, sec.AuthFailed5m, limit)
	return a.evalHostFlag(h, sample, "auth", total >= limit, models.IncidentHostAuth, "HOST_AUTH", msg)
}

func (a *Alerter) evalHostServices(h *models.Host, sample *models.HostSample) error {
	var down []string
	byName := map[string]models.HostServiceStatus{}
	for _, st := range h.ServiceStatus {
		byName[st.Name] = st
	}
	for _, name := range h.Services {
		st, ok := byName[name]
		if !ok || (st.Active != "active" && st.Active != "activating") {
			state := "missing"
			if ok {
				state = st.Active
			}
			down = append(down, name+" ("+state+")")
		}
	}
	msg := "Watched services not active: " + strings.Join(down, ", ")
	return a.evalHostFlag(h, sample, "service", len(down) > 0, models.IncidentHostService, "HOST_SERVICE", msg)
}

func (a *Alerter) evalHostFlag(h *models.Host, sample *models.HostSample, metric string, bad bool, incType models.IncidentType, event, msg string) error {
	open, err := a.store.GetOpenIncident(h.ID, incType)
	if err != nil {
		return err
	}
	if !bad {
		if open == nil {
			return nil
		}
		started := open.StartedAt
		_ = a.store.ResolveOpenIncidents(h.ID, incType, sample.CollectedAt)
		return hostNotify(a, h, AlertMeta{
			Event:      "RECOVERY",
			Message:    "Recovered: " + msg,
			IncidentID: open.ID,
			EventAt:    sample.CollectedAt,
			StartedAt:  &started,
		})
	}
	if open != nil {
		return nil
	}
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
}

func (a *Alerter) evalHostGauge(
	h *models.Host,
	sample *models.HostSample,
	metric string,
	value, warning, critical float64,
	after int,
	incType models.IncidentType,
	event, label, unit, extra string,
) error {
	if after < 1 {
		after = models.DefaultHostCPUAfter
	}
	if warning <= 0 {
		warning = critical
	}
	if critical <= 0 {
		critical = warning
	}
	recoverBelow := warning - 5
	if recoverBelow < 0 {
		recoverBelow = warning * 0.8
		if recoverBelow < 0 {
			recoverBelow = 0
		}
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
	case value >= warning:
		state.ConsecutiveHigh++
		state.ConsecutiveOK = 0
		if err := a.store.UpsertHostAlertState(state); err != nil {
			return err
		}
		if open != nil {
			return nil
		}
		if state.ConsecutiveHigh < after {
			log.Printf("alerter: host %s %s alert pending: %d/%d (value=%.1f warning=%.1f critical=%.1f)", h.DisplayName(), metric, state.ConsecutiveHigh, after, value, warning, critical)
			return nil
		}
		msg := fmt.Sprintf("%s warning: %.1f%s (warning %.1f / critical %.1f) for %d consecutive samples%s",
			label, value, unit, warning, critical, state.ConsecutiveHigh, extra)
		if value >= critical {
			msg = fmt.Sprintf("%s critical: %.1f%s (warning %.1f / critical %.1f) for %d consecutive samples%s",
				label, value, unit, warning, critical, state.ConsecutiveHigh, extra)
		}
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
			Message:    fmt.Sprintf("%s recovered (%.1f%s below %.1f%s)", label, value, unit, recoverBelow, unit),
			IncidentID: open.ID,
			EventAt:    sample.CollectedAt,
			StartedAt:  &started,
		})
	default:
		state.ConsecutiveHigh = 0
		return a.store.UpsertHostAlertState(state)
	}
}
