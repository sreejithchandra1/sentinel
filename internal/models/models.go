package models

import "time"

type MonitorStatus string

const (
	StatusUp       MonitorStatus = "up"
	StatusDown     MonitorStatus = "down"
	StatusDegraded MonitorStatus = "degraded"
	StatusUnknown  MonitorStatus = "unknown"
)

// InvertMonitorStatus swaps up/down when invert mode is enabled (e.g. monitoring that a host should be unreachable).
func InvertMonitorStatus(invert bool, status MonitorStatus) MonitorStatus {
	if !invert {
		return status
	}
	switch status {
	case StatusUp:
		return StatusDown
	case StatusDown:
		return StatusUp
	default:
		return status
	}
}

type MonitorType string

const (
	MonitorHTTP      MonitorType = "http"
	MonitorPort      MonitorType = "port"
	MonitorSSL       MonitorType = "ssl"
	MonitorDNS       MonitorType = "dns"
	MonitorHeartbeat MonitorType = "heartbeat"
)

type IncidentType string

const (
	IncidentDown          IncidentType = "down"
	IncidentSlow          IncidentType = "slow"
	IncidentRecovery      IncidentType = "recovery"
	IncidentSSLExpiry     IncidentType = "ssl_expiry"
	IncidentDNSChange     IncidentType = "dns_change"
	IncidentCertChange    IncidentType = "cert_change"
	IncidentHostOffline   IncidentType = "host_offline"
	IncidentHostCPU       IncidentType = "host_cpu"
	IncidentHostMemory    IncidentType = "host_memory"
	IncidentHostDisk      IncidentType = "host_disk"
	IncidentHostLoad      IncidentType = "host_load"
	IncidentHostSwap      IncidentType = "host_swap"
	IncidentHostIOWait    IncidentType = "host_iowait"
	IncidentHostAuth      IncidentType = "host_auth"
	IncidentHostRootLogin IncidentType = "host_root_login"
	IncidentHostReboot    IncidentType = "host_reboot"
	IncidentHostService   IncidentType = "host_service"
)

type Customer struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MonitorQuota int       `json:"monitor_quota"`
	MonitorCount int       `json:"monitor_count,omitempty"`
	AlertEmails  string    `json:"alert_emails"`
	CreatedAt    time.Time `json:"created_at"`
}

type Monitor struct {
	ID                  string        `json:"id"`
	Type                MonitorType   `json:"type"`
	Name                string        `json:"name"`
	URL                 string        `json:"url"`
	Port                *int          `json:"port,omitempty"`
	Config              string        `json:"config,omitempty"`
	Method              string        `json:"method"`
	ExpectedStatus      int           `json:"expected_status"`
	ExpectedStatusMin   *int          `json:"expected_status_min,omitempty"`
	ExpectedStatusMax   *int          `json:"expected_status_max,omitempty"`
	KeywordMustExist    string        `json:"keyword_must_exist"`
	KeywordMustNotExist string        `json:"keyword_must_not_exist"`
	RequestBody         string        `json:"request_body"`
	RequestHeaders      string        `json:"request_headers"`
	HTTPUsername        string        `json:"http_username,omitempty"`
	HTTPPassword        string        `json:"http_password,omitempty"`
	HTTPAuthSet         bool          `json:"http_auth_set,omitempty"`
	IntervalSeconds     int           `json:"interval_seconds"`
	TimeoutMs           int           `json:"timeout_ms"`
	SlowThresholdMs     int           `json:"slow_threshold_ms"`
	FollowRedirects     bool          `json:"follow_redirects"`
	AlertEmails         string        `json:"alert_emails"`
	Enabled             bool          `json:"enabled"`
	NotifyEmail         bool          `json:"notify_email"`
	NotifySlack         bool          `json:"notify_slack"`
	NotifyWebhooks      bool          `json:"notify_webhooks"`
	Invert              bool          `json:"invert"`
	Tags                []string      `json:"tags"`
	HeartbeatToken      string        `json:"heartbeat_token,omitempty"`
	TenantID            string        `json:"tenant_id,omitempty"`
	AlertAfterFailures  int           `json:"alert_after_failures"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	LastStatus          MonitorStatus `json:"last_status"`
	LastCheckedAt       *time.Time    `json:"last_checked_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type PerformanceTarget struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	URL             string        `json:"url"`
	Method          string        `json:"method"`
	IntervalSeconds int           `json:"interval_seconds"`
	TimeoutMs       int           `json:"timeout_ms"`
	SlowThresholdMs int           `json:"slow_threshold_ms"`
	FollowRedirects bool          `json:"follow_redirects"`
	Enabled         bool          `json:"enabled"`
	AlertEmails     string        `json:"alert_emails"`
	HTTPUsername    string        `json:"http_username,omitempty"`
	HTTPPassword    string        `json:"http_password,omitempty"`
	HTTPAuthSet     bool          `json:"http_auth_set,omitempty"`
	TenantID        string        `json:"tenant_id,omitempty"`
	AlertAfterSlow  int           `json:"alert_after_slow"`
	ConsecutiveSlow int           `json:"consecutive_slow"`
	LastStatus      MonitorStatus `json:"last_status"`
	LastCheckedAt   *time.Time    `json:"last_checked_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type PerformanceResult struct {
	ID             string        `json:"id"`
	TargetID       string        `json:"target_id"`
	Status         MonitorStatus `json:"status"`
	StatusCode     *int          `json:"status_code,omitempty"`
	ResponseTimeMs int           `json:"response_time_ms"`
	DNSMs          *int          `json:"dns_ms,omitempty"`
	TCPMs          *int          `json:"tcp_ms,omitempty"`
	TLSMs          *int          `json:"tls_ms,omitempty"`
	TTFBMs         *int          `json:"ttfb_ms,omitempty"`
	Error          string        `json:"error,omitempty"`
	CheckedAt      time.Time     `json:"checked_at"`
}

type PerformanceTargetListItem struct {
	PerformanceTarget
	LatestResponseTimeMs *int `json:"latest_response_time_ms,omitempty"`
}

type PerformanceStats struct {
	TargetID    string             `json:"target_id"`
	Points      []StatsPoint       `json:"points"`
	AvgResponse int                `json:"avg_response_ms"`
	Performance PerformanceMetrics `json:"performance"`
}

type CheckResult struct {
	ID             string        `json:"id"`
	MonitorID      string        `json:"monitor_id"`
	Status         MonitorStatus `json:"status"`
	StatusCode     *int          `json:"status_code,omitempty"`
	ResponseTimeMs int           `json:"response_time_ms"`
	DNSMs          *int          `json:"dns_ms,omitempty"`
	TCPMs          *int          `json:"tcp_ms,omitempty"`
	TLSMs          *int          `json:"tls_ms,omitempty"`
	TTFBMs         *int          `json:"ttfb_ms,omitempty"`
	Error          string        `json:"error,omitempty"`
	Details        string        `json:"details,omitempty"`
	CheckedAt      time.Time     `json:"checked_at"`
	// ErrorPage is the full captured error response. It is not persisted on
	// check_results (only a compact summary goes in Details); the alerter copies
	// it onto the incident when a DOWN alert fires.
	ErrorPage *HTTPErrorPage `json:"-"`
}

type Incident struct {
	ID             string         `json:"id"`
	MonitorID      string         `json:"monitor_id"`
	Type           IncidentType   `json:"type"`
	Message        string         `json:"message,omitempty"`
	StartedAt      time.Time      `json:"started_at"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
	AcknowledgedAt *time.Time     `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string         `json:"acknowledged_by,omitempty"`
	Details        string         `json:"-"`
	ErrorPage      *HTTPErrorPage `json:"error_page,omitempty"`
}

type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleViewer UserRole = "viewer"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	MFAEnabled   bool      `json:"mfa_enabled"`
	Role         UserRole  `json:"role"`
	TenantID     string    `json:"tenant_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LoginMFAChallenge struct {
	ID                string
	UserID            string
	CodeHash          string
	ExpiresAt         time.Time
	AttemptsRemaining int
	CreatedAt         time.Time
}

type OrgSettings struct {
	CompanyName string `json:"company_name"`
	Tagline     string `json:"tagline"`
	Logo        string `json:"logo"`
}

type SMTPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	From        string `json:"from"`
	AlertEmails string `json:"alert_emails"`
	TLS         bool   `json:"tls"`
	Enabled     bool   `json:"enabled"`
}

type StatsPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	ResponseTimeMs int       `json:"response_time_ms"`
	Status         string    `json:"status"`
	DNSMs          *int      `json:"dns_ms,omitempty"`
	TCPMs          *int      `json:"tcp_ms,omitempty"`
	TLSMs          *int      `json:"tls_ms,omitempty"`
	TTFBMs         *int      `json:"ttfb_ms,omitempty"`
}

type PerformanceMetrics struct {
	AvgMs       int     `json:"avg_ms"`
	MinMs       int     `json:"min_ms"`
	MaxMs       int     `json:"max_ms"`
	P50Ms       int     `json:"p50_ms"`
	P95Ms       int     `json:"p95_ms"`
	P99Ms       int     `json:"p99_ms"`
	SlowCount   int     `json:"slow_count"`
	DegradedPct float64 `json:"degraded_pct"`
}

type MonitorStats struct {
	MonitorID   string             `json:"monitor_id"`
	Points      []StatsPoint       `json:"points"`
	UptimePct   float64            `json:"uptime_pct"`
	AvgResponse int                `json:"avg_response_ms"`
	Performance PerformanceMetrics `json:"performance"`
}

// MonitorRowStats is the compact payload used by the monitors dashboard list.
type MonitorRowStats struct {
	UptimePct float64 `json:"uptime_pct"`
	Points    []int   `json:"points"`
}

type MonitorPerformance struct {
	MonitorID       string `json:"service_id"`
	MonitorName     string `json:"name"`
	Type            string `json:"probe_type"`
	URL             string `json:"target"`
	Status          string `json:"latency_status"`
	SlowThresholdMs int    `json:"slow_threshold_ms"`
	AvgMs           int    `json:"avg_ms"`
	P95Ms           int    `json:"p95_ms"`
	MaxMs           int    `json:"max_ms"`
	SlowCount       int    `json:"slow_count"`
	CheckCount      int    `json:"check_count"`
	HasData         bool   `json:"has_data"`
	Health          string `json:"health"`
}

type FleetTimelinePoint struct {
	Timestamp  time.Time `json:"timestamp"`
	AvgMs      int       `json:"avg_ms"`
	CheckCount int       `json:"check_count"`
	SlowCount  int       `json:"slow_count"`
}

type FleetPerformance struct {
	Period          string               `json:"period"`
	Timeline        []FleetTimelinePoint `json:"timeline"`
	Monitors        []MonitorPerformance `json:"services"`
	TotalChecks     int                  `json:"total_checks"`
	AvgMs           int                  `json:"avg_ms"`
	P95Ms           int                  `json:"p95_ms"`
	SlowChecks      int                  `json:"slow_checks"`
	MonitorCount    int                  `json:"service_count"`
	HealthyCount    int                  `json:"healthy_count"`
	WarningCount    int                  `json:"warning_count"`
	CriticalCount   int                  `json:"critical_count"`
	CollectingCount int                  `json:"collecting_count"`
}

type MonitorListItem struct {
	Monitor
	LatestResponseTimeMs *int `json:"latest_response_time_ms,omitempty"`
}

// Probe detail structs (serialized into CheckResult.Details)

type SSLDetails struct {
	ExpiresAt     string   `json:"expires_at"`
	DaysRemaining int      `json:"days_remaining"`
	Issuer        string   `json:"issuer"`
	Subject       string   `json:"subject"`
	SANs          []string `json:"sans,omitempty"`
	Fingerprint   string   `json:"fingerprint"`
	Issues        []string `json:"issues,omitempty"`
}

type DNSRecordChange struct {
	Type   string `json:"type"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type DNSDetails struct {
	Records map[string][]string `json:"records"`
	Changes []DNSRecordChange   `json:"changes,omitempty"`
}

type PortDetails struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Open bool   `json:"open"`
}

const (
	EmailStatusSent    = "sent"
	EmailStatusFail    = "fail"
	EmailStatusSkip    = "skip"
	EmailStatusPending = "pending"

	EmailKindAlert    = "alert"
	EmailKindTest     = "test"
	EmailKindPassword = "password"
	EmailKindMFA      = "mfa"
)

type EmailLog struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Kind        string    `json:"kind"`
	ToAddr      string    `json:"to_addr"`
	Subject     string    `json:"subject"`
	Error       string    `json:"error,omitempty"`
	MonitorID   string    `json:"monitor_id,omitempty"`
	MonitorName string    `json:"monitor_name,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
