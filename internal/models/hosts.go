package models

import (
	"regexp"
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
	DefaultHostWarning         = 80.0
	DefaultHostCritical        = 90.0
	DefaultHostCPUAfter        = 10 // 5 min at 30s
	DefaultHostMemAfter        = 10
	DefaultHostLoadAfter       = 10
	DefaultHostDiskAfter       = 20 // 10 min at 30s
	DefaultHostAuthFailLimit   = 50
	HostAuthFailWindow         = 5 * time.Minute
	HostEnrollTTL              = 15 * time.Minute
	HostAgentVersion           = "1.1.0"
	MaxHostWatchedServices     = 32
	MaxHostDiskMounts          = 24
)

type HostDisk struct {
	Mount   string  `json:"mount"`
	Percent float64 `json:"percent"`
}

type HostServiceStatus struct {
	Name   string `json:"name"`
	Active string `json:"active"`
	Sub    string `json:"sub,omitempty"`
}

type HostSecurity struct {
	LogsReadable  bool   `json:"logs_readable"`
	SSHFailed5m   int    `json:"ssh_failed_5m"`
	SudoFailed5m  int    `json:"sudo_failed_5m"`
	AuthFailed5m  int    `json:"auth_failed_5m"`
	RootLogins5m  int    `json:"root_logins_5m"`
	LastRootLogin string `json:"last_root_login,omitempty"`
}

type Host struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Hostname           string     `json:"hostname"`
	TenantID           string     `json:"tenant_id,omitempty"`
	OS                 string     `json:"os,omitempty"`
	OSVersion          string     `json:"os_version,omitempty"`
	KernelVersion      string     `json:"kernel_version,omitempty"`
	Arch               string     `json:"arch,omitempty"`
	AgentVersion       string     `json:"agent_version,omitempty"`
	NumCPU             int        `json:"num_cpu,omitempty"`
	RebootRequired     bool       `json:"reboot_required"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	Status             HostStatus `json:"status"`
	Enabled            bool       `json:"enabled"`
	IntervalSeconds    int        `json:"interval_seconds"`
	AlertAfterFailures int        `json:"alert_after_failures"`
	ConsecutiveMisses  int        `json:"consecutive_misses"`

	CollectCPU      bool `json:"collect_cpu"`
	CollectMemory   bool `json:"collect_memory"`
	CollectDisk     bool `json:"collect_disk"`
	CollectLoad     bool `json:"collect_load"`
	CollectSwap     bool `json:"collect_swap"`
	CollectIOWait   bool `json:"collect_iowait"`
	CollectSecurity bool `json:"collect_security"`
	CollectServices bool `json:"collect_services"`

	AlertCPUEnabled      bool    `json:"alert_cpu_enabled"`
	AlertCPUWarning      float64 `json:"alert_cpu_warning"`
	AlertCPUThreshold    float64 `json:"alert_cpu_threshold"`
	AlertCPUAfter        int     `json:"alert_cpu_after"`
	AlertMemoryEnabled   bool    `json:"alert_memory_enabled"`
	AlertMemoryWarning   float64 `json:"alert_memory_warning"`
	AlertMemoryThreshold float64 `json:"alert_memory_threshold"`
	AlertMemoryAfter     int     `json:"alert_memory_after"`
	AlertDiskEnabled     bool    `json:"alert_disk_enabled"`
	AlertDiskWarning     float64 `json:"alert_disk_warning"`
	AlertDiskThreshold   float64 `json:"alert_disk_threshold"`
	AlertDiskAfter       int     `json:"alert_disk_after"`
	AlertLoadEnabled     bool    `json:"alert_load_enabled"`
	AlertLoadWarning     float64 `json:"alert_load_warning"`
	AlertLoadThreshold   float64 `json:"alert_load_threshold"`
	AlertLoadAfter       int     `json:"alert_load_after"`
	AlertSwapEnabled     bool    `json:"alert_swap_enabled"`
	AlertSwapWarning     float64 `json:"alert_swap_warning"`
	AlertSwapThreshold   float64 `json:"alert_swap_threshold"`
	AlertSwapAfter       int     `json:"alert_swap_after"`
	AlertIOWaitEnabled   bool    `json:"alert_iowait_enabled"`
	AlertIOWaitWarning   float64 `json:"alert_iowait_warning"`
	AlertIOWaitThreshold float64 `json:"alert_iowait_threshold"`
	AlertIOWaitAfter     int     `json:"alert_iowait_after"`

	AlertAuthEnabled      bool `json:"alert_auth_enabled"`
	AlertAuthThreshold    int  `json:"alert_auth_threshold"`
	AlertRootLoginEnabled bool `json:"alert_root_login_enabled"`
	AlertRebootEnabled    bool `json:"alert_reboot_enabled"`
	AlertServiceEnabled   bool `json:"alert_service_enabled"`

	Services      []string            `json:"services,omitempty"`
	Disks         []HostDisk          `json:"disks,omitempty"`
	ServiceStatus []HostServiceStatus `json:"service_status,omitempty"`
	Security      *HostSecurity       `json:"security,omitempty"`

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
	ID            string     `json:"id"`
	HostID        string     `json:"host_id"`
	CPUPercent    *float64   `json:"cpu_percent,omitempty"`
	MemPercent    *float64   `json:"mem_percent,omitempty"`
	SwapPercent   *float64   `json:"swap_percent,omitempty"`
	IOWaitPercent *float64   `json:"iowait_percent,omitempty"`
	Load1         *float64   `json:"load1,omitempty"`
	Load5         *float64   `json:"load5,omitempty"`
	Load15        *float64   `json:"load15,omitempty"`
	DiskPercent   *float64   `json:"disk_percent,omitempty"`
	Disks         []HostDisk `json:"disks,omitempty"`
	NumCPU        int        `json:"num_cpu"`
	CollectedAt   time.Time  `json:"collected_at"`
}

type HostAlertState struct {
	HostID          string `json:"host_id"`
	Metric          string `json:"metric"`
	ConsecutiveHigh int    `json:"consecutive_high"`
	ConsecutiveOK   int    `json:"consecutive_ok"`
}

type HostAgentConfig struct {
	IntervalSeconds int      `json:"interval_seconds"`
	CollectCPU      bool     `json:"collect_cpu"`
	CollectMemory   bool     `json:"collect_memory"`
	CollectDisk     bool     `json:"collect_disk"`
	CollectLoad     bool     `json:"collect_load"`
	CollectSwap     bool     `json:"collect_swap"`
	CollectIOWait   bool     `json:"collect_iowait"`
	CollectSecurity bool     `json:"collect_security"`
	CollectServices bool     `json:"collect_services"`
	Services        []string `json:"services,omitempty"`
}

type HostIngestPayload struct {
	Hostname       string              `json:"hostname"`
	OSVersion      string              `json:"os_version"`
	KernelVersion  string              `json:"kernel_version"`
	RebootRequired bool                `json:"reboot_required"`
	CPUPercent     *float64            `json:"cpu_percent"`
	MemPercent     *float64            `json:"mem_percent"`
	SwapPercent    *float64            `json:"swap_percent"`
	IOWaitPercent  *float64            `json:"iowait_percent"`
	Load1          *float64            `json:"load1"`
	Load5          *float64            `json:"load5"`
	Load15         *float64            `json:"load15"`
	NumCPU         int                 `json:"num_cpu"`
	Disks          []HostDisk          `json:"disks"`
	Services       []HostServiceStatus `json:"services"`
	Security       *HostSecurity       `json:"security"`
	AgentVersion   string              `json:"agent_version"`
}

type HostStatsPoint struct {
	Timestamp     time.Time  `json:"timestamp"`
	CPUPercent    *float64   `json:"cpu_percent,omitempty"`
	MemPercent    *float64   `json:"mem_percent,omitempty"`
	SwapPercent   *float64   `json:"swap_percent,omitempty"`
	IOWaitPercent *float64   `json:"iowait_percent,omitempty"`
	Load1         *float64   `json:"load1,omitempty"`
	DiskPercent   *float64   `json:"disk_percent,omitempty"`
	Disks         []HostDisk `json:"disks,omitempty"`
	NumCPU        int        `json:"num_cpu,omitempty"`
}

type HostStats struct {
	HostID string           `json:"host_id"`
	Points []HostStatsPoint `json:"points"`
}

func (h *Host) AgentConfig() HostAgentConfig {
	interval := ClampHostInterval(h.IntervalSeconds)
	svcs := h.Services
	if svcs == nil {
		svcs = []string{}
	}
	return HostAgentConfig{
		IntervalSeconds: interval,
		CollectCPU:      h.CollectCPU,
		CollectMemory:   h.CollectMemory,
		CollectDisk:     h.CollectDisk,
		CollectLoad:     h.CollectLoad,
		CollectSwap:     h.CollectSwap,
		CollectIOWait:   h.CollectIOWait,
		CollectSecurity: h.CollectSecurity,
		CollectServices: h.CollectServices,
		Services:        svcs,
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

func clampWarnCrit(warning, critical float64) (float64, float64) {
	if critical <= 0 {
		critical = DefaultHostCritical
	}
	if warning <= 0 {
		warning = DefaultHostWarning
	}
	if warning >= critical {
		warning = critical - 10
		if warning < 1 {
			warning = critical / 2
		}
	}
	return warning, critical
}

func ApplyHostDefaults(h *Host) {
	if h.Status == "" {
		h.Status = HostPending
	}
	h.IntervalSeconds = ClampHostInterval(h.IntervalSeconds)
	if h.AlertAfterFailures < 1 {
		h.AlertAfterFailures = 2
	}
	h.AlertCPUWarning, h.AlertCPUThreshold = clampWarnCrit(h.AlertCPUWarning, h.AlertCPUThreshold)
	h.AlertMemoryWarning, h.AlertMemoryThreshold = clampWarnCrit(h.AlertMemoryWarning, h.AlertMemoryThreshold)
	h.AlertDiskWarning, h.AlertDiskThreshold = clampWarnCrit(h.AlertDiskWarning, h.AlertDiskThreshold)
	h.AlertSwapWarning, h.AlertSwapThreshold = clampWarnCrit(h.AlertSwapWarning, h.AlertSwapThreshold)
	h.AlertIOWaitWarning, h.AlertIOWaitThreshold = clampWarnCrit(h.AlertIOWaitWarning, h.AlertIOWaitThreshold)
	if h.AlertLoadWarning <= 0 {
		h.AlertLoadWarning = DefaultHostWarning
	}
	if h.AlertLoadThreshold < 0 {
		h.AlertLoadThreshold = 0
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
	if h.AlertSwapAfter < 1 {
		h.AlertSwapAfter = DefaultHostMemAfter
	}
	if h.AlertIOWaitAfter < 1 {
		h.AlertIOWaitAfter = DefaultHostCPUAfter
	}
	if h.AlertAuthThreshold < 1 {
		h.AlertAuthThreshold = DefaultHostAuthFailLimit
	}
	h.Services = NormalizeWatchedServices(h.Services)
}

func HostGrace(h *Host) time.Duration {
	sec := h.IntervalSeconds * 3
	if sec < 90 {
		sec = 90
	}
	return time.Duration(sec) * time.Second
}

func cores(numCPU int) float64 {
	if numCPU < 1 {
		return 1
	}
	return float64(numCPU)
}

// HostLoadWarning is load1 at the warning band (80% of CPU cores by default).
func HostLoadWarning(h *Host, numCPU int) float64 {
	n := cores(numCPU)
	if h.AlertLoadThreshold > 0 {
		w := h.AlertLoadWarning
		if w <= 0 {
			w = DefaultHostWarning
		}
		return h.AlertLoadThreshold * (w / 100)
	}
	w := h.AlertLoadWarning
	if w <= 0 {
		w = DefaultHostWarning
	}
	return n * (w / 100)
}

// HostLoadCritical is load1 at the critical band. 0 threshold means 90% of CPU cores.
func HostLoadCritical(h *Host, numCPU int) float64 {
	if h.AlertLoadThreshold > 0 {
		return h.AlertLoadThreshold
	}
	return cores(numCPU) * (DefaultHostCritical / 100)
}

// HostLoadThreshold is the critical load1 value (kept for older call sites).
func HostLoadThreshold(h *Host, numCPU int) float64 {
	return HostLoadCritical(h, numCPU)
}

var unitNameRe = regexp.MustCompile(`^[A-Za-z0-9:_.\\@-]+$`)

func NormalizeWatchedServices(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		name := strings.TrimSpace(raw)
		name = strings.TrimSuffix(name, ".service")
		if name == "" || !unitNameRe.MatchString(name) || len(name) > 128 {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) >= MaxHostWatchedServices {
			break
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func SanitizeAgentConfig(cfg HostAgentConfig) HostAgentConfig {
	return HostAgentConfig{
		IntervalSeconds: ClampHostInterval(cfg.IntervalSeconds),
		CollectCPU:      cfg.CollectCPU,
		CollectMemory:   cfg.CollectMemory,
		CollectDisk:     cfg.CollectDisk,
		CollectLoad:     cfg.CollectLoad,
		CollectSwap:     cfg.CollectSwap,
		CollectIOWait:   cfg.CollectIOWait,
		CollectSecurity: cfg.CollectSecurity,
		CollectServices: cfg.CollectServices,
		Services:        NormalizeWatchedServices(cfg.Services),
	}
}

func ServiceUnitName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.Contains(name, ".") {
		return name
	}
	return name + ".service"
}
