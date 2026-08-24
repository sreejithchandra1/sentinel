package models

import "time"

type WebhookConfig struct {
	URL     string   `json:"url"`
	Enabled bool     `json:"enabled"`
	Events  []string `json:"events"`
}

// SlackConfig is a tenant-scoped Incoming Webhook integration.
// Platform config uses an empty tenant ID; customers use their tenant ID.
type SlackConfig struct {
	WebhookURL string   `json:"webhook_url"`
	Enabled    bool     `json:"enabled"`
	Events     []string `json:"events"`
}

type MaintenanceWindow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MonitorID string    `json:"monitor_id,omitempty"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditEntry struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type APITokenCreated struct {
	APIToken
	Token string `json:"token"`
}

type ServerSettings struct {
	DashboardURL  string `json:"dashboard_url"`
	RetentionDays int    `json:"retention_days"`
	Workers       int    `json:"workers"`
}

type StatusPageConfig struct {
	Enabled    bool     `json:"enabled"`
	Title      string   `json:"title"`
	MonitorIDs []string `json:"monitor_ids"`
}

type PublicMonitorStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	LastCheck string `json:"last_checked_at,omitempty"`
	URL       string `json:"url,omitempty"`
}

type PublicStatusResponse struct {
	Title    string                `json:"title"`
	Monitors []PublicMonitorStatus `json:"monitors"`
}

type IncidentListItem struct {
	Incident
	MonitorName string `json:"monitor_name"`
}

type SLAMonitorRow struct {
	MonitorID       string   `json:"monitor_id"`
	Name            string   `json:"name"`
	TenantID        string   `json:"tenant_id,omitempty"`
	IncidentCount   int      `json:"incident_count"`
	DowntimeSeconds int64    `json:"downtime_seconds"`
	MTTRSeconds     *float64 `json:"mttr_seconds,omitempty"`
	AvailabilityPct float64  `json:"availability_pct"`
}

type SLAReport struct {
	PeriodStart     time.Time       `json:"period_start"`
	PeriodEnd       time.Time       `json:"period_end"`
	MonitorCount    int             `json:"monitor_count"`
	IncidentCount   int             `json:"incident_count"`
	DowntimeSeconds int64           `json:"downtime_seconds"`
	MTTRSeconds     *float64        `json:"mttr_seconds,omitempty"`
	AvailabilityPct float64         `json:"availability_pct"`
	WindowSeconds   int64           `json:"window_seconds"`
	Monitors        []SLAMonitorRow `json:"monitors"`
}

type HeartbeatConfig struct {
	Token        string `json:"token"`
	GraceSeconds int    `json:"grace_seconds"`
}
