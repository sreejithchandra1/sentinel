package models

import (
	"strings"
	"time"
)

type HostStatus string

const (
	HostPending HostStatus = "pending"
	HostOnline  HostStatus = "online"
	HostOffline HostStatus = "offline"
)

const (
	DefaultHostIntervalSeconds = 30
	MinHostIntervalSeconds     = 30
	MaxHostIntervalSeconds     = 300
	DefaultHostCPUThreshold    = 90.0
	DefaultHostMemThreshold    = 90.0
	DefaultHostDiskThreshold   = 90.0
	DefaultHostCPUAfter        = 10 // 5 min at 30s
	DefaultHostMemAfter        = 10
	DefaultHostLoadAfter       = 10
	DefaultHostDiskAfter       = 20 // 10 min at 30s
	HostEnrollTTL              = 15 * time.Minute
	HostAgentVersion           = "1.0.0"
)

type HostDisk struct {
	Mount   string  `json:"mount"`
	Percent float64 `json:"percent"`
}

type Host struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Hostname           string     `json:"hostname"`
	TenantID           string     `json:"tenant_id,omitempty"`
	OS                 string     `json:"os,omitempty"`
	Arch               string     `json:"arch,omitempty"`
	AgentVersion       string     `json:"agent_version,omitempty"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	Status             HostStatus `json:"status"`
	Enabled            bool       `json:"enabled"`
	IntervalSeconds    int        `json:"interval_seconds"`
	AlertAfterFailures int        `json:"alert_after_failures"`
	ConsecutiveMisses  int        `json:"consecutive_misses"`

	CollectCPU    bool `json:"collect_cpu"`
	CollectMemory bool `json:"collect_memory"`
	CollectDisk   bool `json:"collect_disk"`
	CollectLoad   bool `json:"collect_load"`

	AlertCPUEnabled      bool    `json:"alert_cpu_enabled"`
	AlertCPUThreshold    float64 `json:"alert_cpu_threshold"`
	AlertCPUAfter        int     `json:"alert_cpu_after"`
	AlertMemoryEnabled   bool    `json:"alert_memory_enabled"`
	AlertMemoryThreshold float64 `json:"alert_memory_threshold"`
	AlertMemoryAfter     int     `json:"alert_memory_after"`
	AlertDiskEnabled     bool    `json:"alert_disk_enabled"`
	AlertDiskThreshold   float64 `json:"alert_disk_threshold"`
	AlertDiskAfter       int     `json:"alert_disk_after"`
	AlertLoadEnabled     bool    `json:"alert_load_enabled"`
	AlertLoadThreshold   float64 `json:"alert_load_threshold"`
	AlertLoadAfter       int     `json:"alert_load_after"`

	AlertEmails    string `json:"alert_emails"`
	NotifyEmail    bool   `json:"notify_email"`
	NotifySlack    bool   `json:"notify_slack"`
	NotifyWebhooks bool   `json:"notify_webhooks"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	EnrollToken    string     `json:"enroll_token,omitempty"`
	InstallCommand string     `json:"install_command,omitempty"`
	EnrollExpires  *time.Time `json:"enroll_expires_at,omitempty"`
}

func (h *Host) DisplayName() string {
	if h == nil {
		return "host"
	}
	if n := strings.TrimSpace(h.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(h.Hostname); n != "" {
		return n
	}
	return "New host"
}

type HostSample struct {
	ID          string     `json:"id"`
	HostID      string     `json:"host_id"`
	CPUPercent  *float64   `json:"cpu_percent,omitempty"`
	MemPercent  *float64   `json:"mem_percent,omitempty"`
	Load1       *float64   `json:"load1,omitempty"`
	Load5       *float64   `json:"load5,omitempty"`
	Load15      *float64   `json:"load15,omitempty"`
	DiskPercent *float64   `json:"disk_percent,omitempty"`
	Disks       []HostDisk `json:"disks,omitempty"`
	NumCPU      int        `json:"num_cpu"`
	CollectedAt time.Time  `json:"collected_at"`
}

type HostAlertState struct {
	HostID          string `json:"host_id"`
	Metric          string `json:"metric"`
	ConsecutiveHigh int    `json:"consecutive_high"`
	ConsecutiveOK   int    `json:"consecutive_ok"`
}

type HostAgentConfig struct {
	IntervalSeconds int  `json:"interval_seconds"`
	CollectCPU      bool `json:"collect_cpu"`
	CollectMemory   bool `json:"collect_memory"`
	CollectDisk     bool `json:"collect_disk"`
	CollectLoad     bool `json:"collect_load"`
}

type HostIngestPayload struct {
	Hostname     string     `json:"hostname"`
	CPUPercent   *float64   `json:"cpu_percent"`
	MemPercent   *float64   `json:"mem_percent"`
	Load1        *float64   `json:"load1"`
	Load5        *float64   `json:"load5"`
	Load15       *float64   `json:"load15"`
	NumCPU       int        `json:"num_cpu"`
	Disks        []HostDisk `json:"disks"`
	AgentVersion string     `json:"agent_version"`
}

type HostStatsPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  *float64  `json:"cpu_percent,omitempty"`
	MemPercent  *float64  `json:"mem_percent,omitempty"`
	Load1       *float64  `json:"load1,omitempty"`
	DiskPercent *float64  `json:"disk_percent,omitempty"`
}

type HostStats struct {
	HostID string           `json:"host_id"`
	Points []HostStatsPoint `json:"points"`
}

func (h *Host) AgentConfig() HostAgentConfig {
	interval := h.IntervalSeconds
	if interval < MinHostIntervalSeconds {
		interval = DefaultHostIntervalSeconds
	}
	if interval > MaxHostIntervalSeconds {
		interval = MaxHostIntervalSeconds
	}
	return HostAgentConfig{
		IntervalSeconds: interval,
		CollectCPU:      h.CollectCPU,
		CollectMemory:   h.CollectMemory,
		CollectDisk:     h.CollectDisk,
		CollectLoad:     h.CollectLoad,
	}
}

func ClampHostInterval(n int) int {
	if n < MinHostIntervalSeconds {
		return DefaultHostIntervalSeconds
	}
	if n > MaxHostIntervalSeconds {
		return MaxHostIntervalSeconds
	}
	return n
}

func ApplyHostDefaults(h *Host) {
	if h.Status == "" {
		h.Status = HostPending
	}
	h.IntervalSeconds = ClampHostInterval(h.IntervalSeconds)
	if h.AlertAfterFailures < 1 {
		h.AlertAfterFailures = 2
	}
	if h.AlertCPUThreshold <= 0 {
		h.AlertCPUThreshold = DefaultHostCPUThreshold
	}
	if h.AlertMemoryThreshold <= 0 {
		h.AlertMemoryThreshold = DefaultHostMemThreshold
	}
	if h.AlertDiskThreshold <= 0 {
		h.AlertDiskThreshold = DefaultHostDiskThreshold
	}
	if h.AlertCPUAfter < 1 {
		h.AlertCPUAfter = DefaultHostCPUAfter
	}
	if h.AlertMemoryAfter < 1 {
		h.AlertMemoryAfter = DefaultHostMemAfter
	}
	if h.AlertLoadAfter < 1 {
		h.AlertLoadAfter = DefaultHostLoadAfter
	}
	if h.AlertDiskAfter < 1 {
		h.AlertDiskAfter = DefaultHostDiskAfter
	}
}

func HostGrace(h *Host) time.Duration {
	sec := h.IntervalSeconds * 3
	if sec < 90 {
		sec = 90
	}
	return time.Duration(sec) * time.Second
}

func HostLoadThreshold(h *Host, numCPU int) float64 {
	if h.AlertLoadThreshold > 0 {
		return h.AlertLoadThreshold
	}
	if numCPU < 1 {
		numCPU = 1
	}
	return float64(numCPU) * 2
}
