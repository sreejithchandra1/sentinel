package alerter

import (
	"fmt"
	"strings"
	"time"
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
}

func (m AlertMeta) Title() string {
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN":
		return "MONITOR DOWN"
	case "RECOVERY":
		return "MONITOR RECOVERED"
	case "SLOW":
		return "PERFORMANCE SLOW"
	case "NORMAL":
		return "BACK TO NORMAL"
	case "HOST_OFFLINE":
		return "HOST OFFLINE"
	case "HOST_CPU":
		return "HOST CPU HIGH"
	case "HOST_MEMORY":
		return "HOST MEMORY HIGH"
	case "HOST_DISK":
		return "HOST DISK HIGH"
	case "HOST_LOAD":
		return "HOST LOAD HIGH"
	case "HOST_SWAP":
		return "HOST SWAP HIGH"
	case "HOST_IOWAIT":
		return "HOST DISK IO WAIT"
	case "HOST_AUTH":
		return "HOST AUTH FAILURES"
	case "HOST_ROOT_LOGIN":
		return "HOST ROOT LOGIN"
	case "HOST_REBOOT":
		return "HOST REBOOT REQUIRED"
	case "HOST_SERVICE":
		return "HOST SERVICE DOWN"
	default:
		return strings.ToUpper(m.Event)
	}
}

func (m AlertMeta) Color() string {
	switch strings.ToUpper(strings.TrimSpace(m.Event)) {
	case "DOWN", "SLOW", "HOST_OFFLINE", "HOST_CPU", "HOST_MEMORY", "HOST_DISK", "HOST_LOAD",
		"HOST_SWAP", "HOST_IOWAIT", "HOST_AUTH", "HOST_ROOT_LOGIN", "HOST_REBOOT", "HOST_SERVICE":
		return "#E01E5A"
	case "RECOVERY", "NORMAL":
		return "#2EB67D"
	default:
		return "#1D9BD1"
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
		return "HIGH"
	case "HOST_AUTH", "HOST_ROOT_LOGIN", "HOST_REBOOT", "HOST_SERVICE":
		return "ALERT"
	default:
		return strings.ToUpper(m.Event)
	}
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
	if m.StartedAt == nil {
		return ""
	}
	end := m.EventAt
	if end.IsZero() {
		end = time.Now().UTC()
	}
	d := end.Sub(*m.StartedAt)
	if d < 0 {
		return ""
	}
	return formatShortDuration(d)
}

func (m AlertMeta) FallbackText() string {
	name := m.Name
	if name == "" {
		name = "monitor"
	}
	return fmt.Sprintf("[Sentinel] %s: %s", strings.ToUpper(m.Event), name)
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
