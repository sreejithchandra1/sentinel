package alerter

import (
	"fmt"
	"strings"
	"time"
)

const (
	alertColorDanger  = "#E01E5A"
	alertColorOK      = "#2EB67D"
	alertColorInfo    = "#1D9BD1"
	alertColorWarning = "#F59E0B"
)

// AlertMeta carries shared fields for Slack and email alert rendering.
type AlertMeta struct {
	Event        string
	Name         string
	URL          string
	Message      string
	DashboardURL string
	ResponseMs   int
	IncidentID   string
	EventAt      time.Time
	StartedAt    *time.Time // for downtime on recovery
	// Severity is "warning" or "critical" for host gauge alerts. Empty means infer from Message.
	Severity string
}

func (m AlertMeta) Title() string {
	var title string
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN":
		title = "Outage Detected"
	case "RECOVERY":
		title = m.recoveredAfterTitle()
	case "SLOW":
		title = "PERFORMANCE SLOW"
	case "NORMAL":
		title = "BACK TO NORMAL"
	case "HOST_OFFLINE":
		title = "HOST OFFLINE"
	case "HOST_CPU":
		title = "HOST CPU HIGH"
	case "HOST_MEMORY":
		title = "HOST MEMORY HIGH"
	case "HOST_DISK":
		title = "HOST DISK HIGH"
	case "HOST_LOAD":
		title = "HOST LOAD HIGH"
	case "HOST_SWAP":
		title = "HOST SWAP HIGH"
	case "HOST_IOWAIT":
		title = "HOST DISK IO WAIT"
	case "HOST_AUTH":
		title = "HOST AUTH FAILURES"
	case "HOST_ROOT_LOGIN":
		title = "HOST ROOT LOGIN"
	case "HOST_REBOOT":
		title = "HOST REBOOT REQUIRED"
	case "HOST_SERVICE":
		title = "HOST SERVICE DOWN"
	default:
		title = strings.ToUpper(m.Event)
	}
	if m.isWarningGauge() {
		title = strings.Replace(title, " HIGH", " WARNING", 1)
	}
	return title
}

func (m AlertMeta) Color() string {
	if m.isWarningGauge() {
		return alertColorWarning
	}
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN", "SLOW", "HOST_OFFLINE", "HOST_CPU", "HOST_MEMORY", "HOST_DISK", "HOST_LOAD",
		"HOST_SWAP", "HOST_IOWAIT", "HOST_AUTH", "HOST_ROOT_LOGIN", "HOST_REBOOT", "HOST_SERVICE":
		return alertColorDanger
	case "RECOVERY", "NORMAL":
		return alertColorOK
	default:
		return alertColorInfo
	}
}

func (m AlertMeta) StatusLabel() string {
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN":
		return "DOWN"
	case "RECOVERY":
		return "UP"
	case "SLOW":
		return "SLOW"
	case "NORMAL":
		return "OK"
	case "HOST_OFFLINE":
		return "OFFLINE"
	case "HOST_CPU", "HOST_MEMORY", "HOST_DISK", "HOST_LOAD", "HOST_SWAP", "HOST_IOWAIT":
		if m.isWarningGauge() {
			return "WARNING"
		}
		return "HIGH"
	case "HOST_AUTH", "HOST_ROOT_LOGIN", "HOST_REBOOT", "HOST_SERVICE":
		return "ALERT"
	default:
		return strings.ToUpper(m.Event)
	}
}

func isHostGaugeEvent(event string) bool {
	switch strings.ToUpper(strings.TrimSpace(event)) {
	case "HOST_CPU", "HOST_MEMORY", "HOST_DISK", "HOST_LOAD", "HOST_SWAP", "HOST_IOWAIT":
		return true
	default:
		return false
	}
}

// isWarningGauge reports a host CPU/disk/etc alert that has not reached the critical threshold.
func (m AlertMeta) isWarningGauge() bool {
	if !isHostGaugeEvent(m.Event) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(m.Severity)) {
	case "warning":
		return true
	case "critical":
		return false
	}
	lower := strings.ToLower(m.Message)
	if strings.Contains(lower, "critical:") {
		return false
	}
	return strings.Contains(lower, "warning:")
}

func (m AlertMeta) ResponseLabel() string {
	lower := strings.ToLower(m.Message)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") || strings.Contains(lower, "timed out") {
		return "Timeout"
	}
	if m.ResponseMs <= 0 {
		return "—"
	}
	return fmt.Sprintf("%dms", m.ResponseMs)
}

func (m AlertMeta) IncidentLabel() string {
	id := strings.TrimSpace(m.IncidentID)
	if id == "" {
		return "—"
	}
	if len(id) > 8 {
		id = id[:8]
	}
	return "INC-" + strings.ToUpper(id)
}

func (m AlertMeta) EventTimeLabel() string {
	t := m.EventAt
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format("02 Jan 2006 03:04 PM") + " UTC"
}

func (m AlertMeta) TimeFieldLabel() string {
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "RECOVERY", "NORMAL":
		return "Recovered"
	default:
		return "Detected"
	}
}

func (m AlertMeta) DowntimeLabel() string {
	d, ok := m.downtimeDuration()
	if !ok {
		return ""
	}
	return formatShortDuration(d)
}

func (m AlertMeta) FallbackText() string {
	name := m.Name
	if name == "" {
		name = "monitor"
	}
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN":
		return "Outage Detected: " + name
	case "RECOVERY":
		return m.recoveredAfterTitle() + ": " + name
	default:
		return fmt.Sprintf("%s: %s", strings.ToUpper(m.Event), name)
	}
}

func (m AlertMeta) downtimeDuration() (time.Duration, bool) {
	if m.StartedAt == nil {
		return 0, false
	}
	end := m.EventAt
	if end.IsZero() {
		end = time.Now().UTC()
	}
	d := end.Sub(*m.StartedAt)
	if d < 0 {
		return 0, false
	}
	return d, true
}

func (m AlertMeta) recoveredAfterTitle() string {
	d, ok := m.downtimeDuration()
	if !ok {
		return "Recovered"
	}
	return formatRecoveredAfter(d)
}

func formatRecoveredAfter(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d >= time.Hour {
		hours := int(d.Round(time.Hour) / time.Hour)
		if hours < 1 {
			hours = 1
		}
		if hours == 1 {
			return "Recovered after 1 hour"
		}
		return fmt.Sprintf("Recovered after %d hours", hours)
	}
	minutes := int(d.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	if minutes >= 60 {
		return "Recovered after 1 hour"
	}
	if minutes == 1 {
		return "Recovered after 1 minute"
	}
	return fmt.Sprintf("Recovered after %d minutes", minutes)
}

func formatShortDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
