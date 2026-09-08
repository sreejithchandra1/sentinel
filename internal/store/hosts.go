package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const hostColumns = `id, name, hostname, tenant_id, os, os_version, kernel_version, arch, agent_version,
num_cpu, reboot_required, last_seen_at, status,
enabled, interval_seconds, alert_after_failures, consecutive_misses,
collect_cpu, collect_memory, collect_disk, collect_load, collect_swap, collect_iowait, collect_security, collect_services,
alert_cpu_enabled, alert_cpu_warning, alert_cpu_threshold, alert_cpu_after,
alert_memory_enabled, alert_memory_warning, alert_memory_threshold, alert_memory_after,
alert_disk_enabled, alert_disk_warning, alert_disk_threshold, alert_disk_after,
alert_load_enabled, alert_load_warning, alert_load_threshold, alert_load_after,
alert_swap_enabled, alert_swap_warning, alert_swap_threshold, alert_swap_after,
alert_iowait_enabled, alert_iowait_warning, alert_iowait_threshold, alert_iowait_after,
alert_auth_enabled, alert_auth_threshold, alert_root_login_enabled, alert_reboot_enabled, alert_service_enabled,
services_json, security_json, disks_latest_json, services_latest_json,
alert_emails, notify_email, notify_slack, notify_webhooks,
created_at, updated_at`

type hostScanRow struct {
	id, name, hostname, tenantID, os, osVersion, kernel, arch, agentVersion, status string
	lastSeenAt, createdAt, updatedAt                                                sql.NullString
	numCPU, rebootRequired                                                          int
	enabled                                                                         int
	collectCPU, collectMemory, collectDisk, collectLoad                             int
	collectSwap, collectIOWait, collectSecurity, collectServices                    int
	notifyEmail, notifySlack, notifyWebhooks                                        int
	alertCPU, alertMem, alertDisk, alertLoad, alertSwap, alertIOWait                int
	alertAuth, alertRoot, alertReboot, alertService                                 int
	intervalSeconds, alertAfter, consecutiveMisses                                  int
	alertCPUAfter, alertMemAfter, alertDiskAfter, alertLoadAfter                    int
	alertSwapAfter, alertIOWaitAfter, alertAuthThresh                               int
	alertCPUWarn, alertCPUThresh, alertMemWarn, alertMemThresh                      float64
	alertDiskWarn, alertDiskThresh, alertLoadWarn, alertLoadThresh                  float64
	alertSwapWarn, alertSwapThresh, alertIOWaitWarn, alertIOWaitThresh              float64
	alertEmails                                                                     string
	servicesJSON, securityJSON, disksJSON, servicesLatestJSON                       string
}

func (r *hostScanRow) scan(row interface{ Scan(dest ...any) error }) error {
	return row.Scan(
		&r.id, &r.name, &r.hostname, &r.tenantID, &r.os, &r.osVersion, &r.kernel, &r.arch, &r.agentVersion,
		&r.numCPU, &r.rebootRequired, &r.lastSeenAt, &r.status,
		&r.enabled, &r.intervalSeconds, &r.alertAfter, &r.consecutiveMisses,
		&r.collectCPU, &r.collectMemory, &r.collectDisk, &r.collectLoad,
		&r.collectSwap, &r.collectIOWait, &r.collectSecurity, &r.collectServices,
		&r.alertCPU, &r.alertCPUWarn, &r.alertCPUThresh, &r.alertCPUAfter,
		&r.alertMem, &r.alertMemWarn, &r.alertMemThresh, &r.alertMemAfter,
		&r.alertDisk, &r.alertDiskWarn, &r.alertDiskThresh, &r.alertDiskAfter,
		&r.alertLoad, &r.alertLoadWarn, &r.alertLoadThresh, &r.alertLoadAfter,
		&r.alertSwap, &r.alertSwapWarn, &r.alertSwapThresh, &r.alertSwapAfter,
		&r.alertIOWait, &r.alertIOWaitWarn, &r.alertIOWaitThresh, &r.alertIOWaitAfter,
		&r.alertAuth, &r.alertAuthThresh, &r.alertRoot, &r.alertReboot, &r.alertService,
		&r.servicesJSON, &r.securityJSON, &r.disksJSON, &r.servicesLatestJSON,
		&r.alertEmails, &r.notifyEmail, &r.notifySlack, &r.notifyWebhooks,
		&r.createdAt, &r.updatedAt,
	)
}

func (r *hostScanRow) toHost() models.Host {
	h := models.Host{
		ID:                    r.id,
		Name:                  r.name,
		Hostname:              r.hostname,
		TenantID:              r.tenantID,
		OS:                    r.os,
		OSVersion:             r.osVersion,
		KernelVersion:         r.kernel,
		Arch:                  r.arch,
		AgentVersion:          r.agentVersion,
		NumCPU:                r.numCPU,
		RebootRequired:        intToBool(r.rebootRequired),
		LastSeenAt:            nullableTime(r.lastSeenAt),
		Status:                models.HostStatus(r.status),
		Enabled:               intToBool(r.enabled),
		IntervalSeconds:       r.intervalSeconds,
		AlertAfterFailures:    r.alertAfter,
		ConsecutiveMisses:     r.consecutiveMisses,
		CollectCPU:            intToBool(r.collectCPU),
		CollectMemory:         intToBool(r.collectMemory),
		CollectDisk:           intToBool(r.collectDisk),
		CollectLoad:           intToBool(r.collectLoad),
		CollectSwap:           intToBool(r.collectSwap),
		CollectIOWait:         intToBool(r.collectIOWait),
		CollectSecurity:       intToBool(r.collectSecurity),
		CollectServices:       intToBool(r.collectServices),
		AlertCPUEnabled:       intToBool(r.alertCPU),
		AlertCPUWarning:       r.alertCPUWarn,
		AlertCPUThreshold:     r.alertCPUThresh,
		AlertCPUAfter:         r.alertCPUAfter,
		AlertMemoryEnabled:    intToBool(r.alertMem),
		AlertMemoryWarning:    r.alertMemWarn,
		AlertMemoryThreshold:  r.alertMemThresh,
		AlertMemoryAfter:      r.alertMemAfter,
		AlertDiskEnabled:      intToBool(r.alertDisk),
		AlertDiskWarning:      r.alertDiskWarn,
		AlertDiskThreshold:    r.alertDiskThresh,
		AlertDiskAfter:        r.alertDiskAfter,
		AlertLoadEnabled:      intToBool(r.alertLoad),
		AlertLoadWarning:      r.alertLoadWarn,
		AlertLoadThreshold:    r.alertLoadThresh,
		AlertLoadAfter:        r.alertLoadAfter,
		AlertSwapEnabled:      intToBool(r.alertSwap),
		AlertSwapWarning:      r.alertSwapWarn,
		AlertSwapThreshold:    r.alertSwapThresh,
		AlertSwapAfter:        r.alertSwapAfter,
		AlertIOWaitEnabled:    intToBool(r.alertIOWait),
		AlertIOWaitWarning:    r.alertIOWaitWarn,
		AlertIOWaitThreshold:  r.alertIOWaitThresh,
		AlertIOWaitAfter:      r.alertIOWaitAfter,
		AlertAuthEnabled:      intToBool(r.alertAuth),
		AlertAuthThreshold:    r.alertAuthThresh,
		AlertRootLoginEnabled: intToBool(r.alertRoot),
		AlertRebootEnabled:    intToBool(r.alertReboot),
		AlertServiceEnabled:   intToBool(r.alertService),
		Services:              unmarshalStringSlice(r.servicesJSON),
		Disks:                 unmarshalDisks(r.disksJSON),
		ServiceStatus:         unmarshalServiceStatus(r.servicesLatestJSON),
		Security:              unmarshalSecurity(r.securityJSON),
		AlertEmails:           r.alertEmails,
		NotifyEmail:           intToBool(r.notifyEmail),
		NotifySlack:           intToBool(r.notifySlack),
		NotifyWebhooks:        intToBool(r.notifyWebhooks),
	}
	if t, err := parseTime(nullableString(r.createdAt)); err == nil {
		h.CreatedAt = t
	}
	if t, err := parseTime(nullableString(r.updatedAt)); err == nil {
		h.UpdatedAt = t
	}
	return h
}

func (s *Store) CreateHost(h *models.Host) error {
	models.ApplyHostDefaults(h)
	if h.ID == "" {
		h.ID = newID()
	}
	now := time.Now().UTC()
	h.CreatedAt = now
	h.UpdatedAt = now
	if !h.CollectCPU && !h.CollectMemory && !h.CollectDisk && !h.CollectLoad && !h.CollectSwap && !h.CollectIOWait {
		h.CollectCPU, h.CollectMemory, h.CollectDisk, h.CollectLoad = true, true, true, true
		h.CollectSwap, h.CollectIOWait, h.CollectSecurity, h.CollectServices = true, true, true, true
	}
	_, err := s.db.Exec(`
		INSERT INTO hosts (
			id, name, hostname, tenant_id, os, os_version, kernel_version, arch, agent_version, token_hash,
			num_cpu, reboot_required, last_seen_at, status,
			enabled, interval_seconds, alert_after_failures, consecutive_misses,
			collect_cpu, collect_memory, collect_disk, collect_load, collect_swap, collect_iowait, collect_security, collect_services,
			alert_cpu_enabled, alert_cpu_warning, alert_cpu_threshold, alert_cpu_after,
			alert_memory_enabled, alert_memory_warning, alert_memory_threshold, alert_memory_after,
			alert_disk_enabled, alert_disk_warning, alert_disk_threshold, alert_disk_after,
			alert_load_enabled, alert_load_warning, alert_load_threshold, alert_load_after,
			alert_swap_enabled, alert_swap_warning, alert_swap_threshold, alert_swap_after,
			alert_iowait_enabled, alert_iowait_warning, alert_iowait_threshold, alert_iowait_after,
			alert_auth_enabled, alert_auth_threshold, alert_root_login_enabled, alert_reboot_enabled, alert_service_enabled,
			services_json, security_json, disks_latest_json, services_latest_json,
			alert_emails, notify_email, notify_slack, notify_webhooks,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.Hostname, h.TenantID, h.OS, h.OSVersion, h.KernelVersion, h.Arch, h.AgentVersion,
		h.NumCPU, boolToInt(h.RebootRequired), nullableTimeArg(h.LastSeenAt), string(h.Status),
		boolToInt(h.Enabled), h.IntervalSeconds, h.AlertAfterFailures, h.ConsecutiveMisses,
		boolToInt(h.CollectCPU), boolToInt(h.CollectMemory), boolToInt(h.CollectDisk), boolToInt(h.CollectLoad),
		boolToInt(h.CollectSwap), boolToInt(h.CollectIOWait), boolToInt(h.CollectSecurity), boolToInt(h.CollectServices),
		boolToInt(h.AlertCPUEnabled), h.AlertCPUWarning, h.AlertCPUThreshold, h.AlertCPUAfter,
		boolToInt(h.AlertMemoryEnabled), h.AlertMemoryWarning, h.AlertMemoryThreshold, h.AlertMemoryAfter,
		boolToInt(h.AlertDiskEnabled), h.AlertDiskWarning, h.AlertDiskThreshold, h.AlertDiskAfter,
		boolToInt(h.AlertLoadEnabled), h.AlertLoadWarning, h.AlertLoadThreshold, h.AlertLoadAfter,
		boolToInt(h.AlertSwapEnabled), h.AlertSwapWarning, h.AlertSwapThreshold, h.AlertSwapAfter,
		boolToInt(h.AlertIOWaitEnabled), h.AlertIOWaitWarning, h.AlertIOWaitThreshold, h.AlertIOWaitAfter,
		boolToInt(h.AlertAuthEnabled), h.AlertAuthThreshold, boolToInt(h.AlertRootLoginEnabled), boolToInt(h.AlertRebootEnabled), boolToInt(h.AlertServiceEnabled),
		marshalJSON(h.Services), marshalJSON(h.Security), marshalJSON(h.Disks), marshalJSON(h.ServiceStatus),
		h.AlertEmails, boolToInt(h.NotifyEmail), boolToInt(h.NotifySlack), boolToInt(h.NotifyWebhooks),
		formatTime(h.CreatedAt), formatTime(h.UpdatedAt),
	)
	return err
}

func nullableTimeArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func (s *Store) GetHost(id string) (*models.Host, error) {
	row := s.db.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE id = ?`, id)
	var r hostScanRow
	if err := r.scan(row); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	h := r.toHost()
	return &h, nil
}

func (s *Store) GetHostByIngestToken(token string) (*models.Host, error) {
	if token == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE token_hash = ?`, hashAPIToken(token))
	var r hostScanRow
	if err := r.scan(row); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	h := r.toHost()
	return &h, nil
}

func (s *Store) ListHosts() ([]models.Host, error) {
	return s.listHosts(`SELECT ` + hostColumns + ` FROM hosts ORDER BY name, hostname, created_at`)
}

func (s *Store) ListHostsByTenant(tenantID string) ([]models.Host, error) {
	return s.listHosts(`SELECT `+hostColumns+` FROM hosts WHERE tenant_id = ? ORDER BY name, hostname, created_at`, tenantID)
}

func (s *Store) ListEnabledHosts() ([]models.Host, error) {
	return s.listHosts(`SELECT ` + hostColumns + ` FROM hosts WHERE enabled = 1`)
}

func (s *Store) listHosts(query string, args ...any) ([]models.Host, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Host
	for rows.Next() {
		var r hostScanRow
		if err := r.scan(rows); err != nil {
			return nil, err
		}
		out = append(out, r.toHost())
	}
	if out == nil {
		out = []models.Host{}
	}
	return out, rows.Err()
}

func (s *Store) UpdateHost(h *models.Host) error {
	models.ApplyHostDefaults(h)
	h.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE hosts SET
			name = ?, hostname = ?, tenant_id = ?, os = ?, os_version = ?, kernel_version = ?, arch = ?, agent_version = ?,
			num_cpu = ?, reboot_required = ?, last_seen_at = ?, status = ?, enabled = ?, interval_seconds = ?,
			alert_after_failures = ?, consecutive_misses = ?,
			collect_cpu = ?, collect_memory = ?, collect_disk = ?, collect_load = ?, collect_swap = ?, collect_iowait = ?, collect_security = ?, collect_services = ?,
			alert_cpu_enabled = ?, alert_cpu_warning = ?, alert_cpu_threshold = ?, alert_cpu_after = ?,
			alert_memory_enabled = ?, alert_memory_warning = ?, alert_memory_threshold = ?, alert_memory_after = ?,
			alert_disk_enabled = ?, alert_disk_warning = ?, alert_disk_threshold = ?, alert_disk_after = ?,
			alert_load_enabled = ?, alert_load_warning = ?, alert_load_threshold = ?, alert_load_after = ?,
			alert_swap_enabled = ?, alert_swap_warning = ?, alert_swap_threshold = ?, alert_swap_after = ?,
			alert_iowait_enabled = ?, alert_iowait_warning = ?, alert_iowait_threshold = ?, alert_iowait_after = ?,
			alert_auth_enabled = ?, alert_auth_threshold = ?, alert_root_login_enabled = ?, alert_reboot_enabled = ?, alert_service_enabled = ?,
			services_json = ?, security_json = ?, disks_latest_json = ?, services_latest_json = ?,
			alert_emails = ?, notify_email = ?, notify_slack = ?, notify_webhooks = ?,
			updated_at = ?
		WHERE id = ?`,
		h.Name, h.Hostname, h.TenantID, h.OS, h.OSVersion, h.KernelVersion, h.Arch, h.AgentVersion,
		h.NumCPU, boolToInt(h.RebootRequired), nullableTimeArg(h.LastSeenAt), string(h.Status), boolToInt(h.Enabled), h.IntervalSeconds,
		h.AlertAfterFailures, h.ConsecutiveMisses,
		boolToInt(h.CollectCPU), boolToInt(h.CollectMemory), boolToInt(h.CollectDisk), boolToInt(h.CollectLoad),
		boolToInt(h.CollectSwap), boolToInt(h.CollectIOWait), boolToInt(h.CollectSecurity), boolToInt(h.CollectServices),
		boolToInt(h.AlertCPUEnabled), h.AlertCPUWarning, h.AlertCPUThreshold, h.AlertCPUAfter,
		boolToInt(h.AlertMemoryEnabled), h.AlertMemoryWarning, h.AlertMemoryThreshold, h.AlertMemoryAfter,
		boolToInt(h.AlertDiskEnabled), h.AlertDiskWarning, h.AlertDiskThreshold, h.AlertDiskAfter,
		boolToInt(h.AlertLoadEnabled), h.AlertLoadWarning, h.AlertLoadThreshold, h.AlertLoadAfter,
		boolToInt(h.AlertSwapEnabled), h.AlertSwapWarning, h.AlertSwapThreshold, h.AlertSwapAfter,
		boolToInt(h.AlertIOWaitEnabled), h.AlertIOWaitWarning, h.AlertIOWaitThreshold, h.AlertIOWaitAfter,
		boolToInt(h.AlertAuthEnabled), h.AlertAuthThreshold, boolToInt(h.AlertRootLoginEnabled), boolToInt(h.AlertRebootEnabled), boolToInt(h.AlertServiceEnabled),
		marshalJSON(h.Services), marshalJSON(h.Security), marshalJSON(h.Disks), marshalJSON(h.ServiceStatus),
		h.AlertEmails, boolToInt(h.NotifyEmail), boolToInt(h.NotifySlack), boolToInt(h.NotifyWebhooks),
		formatTime(h.UpdatedAt), h.ID,
	)
	return err
}

func (s *Store) SetHostEnabled(id string, enabled bool) error {
	_, err := s.db.Exec(`UPDATE hosts SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), formatTime(time.Now().UTC()), id)
	return err
}

func (s *Store) SetHostIngestToken(id, token string) error {
	_, err := s.db.Exec(`UPDATE hosts SET token_hash = ?, updated_at = ? WHERE id = ?`,
		hashAPIToken(token), formatTime(time.Now().UTC()), id)
	return err
}

func (s *Store) TouchHostSeen(id string, hostname, osName, arch, agentVersion string, numCPU int, at time.Time) error {
	_, err := s.db.Exec(`
		UPDATE hosts SET
			hostname = CASE WHEN ? != '' THEN ? ELSE hostname END,
			os = CASE WHEN ? != '' THEN ? ELSE os END,
			arch = CASE WHEN ? != '' THEN ? ELSE arch END,
			agent_version = CASE WHEN ? != '' THEN ? ELSE agent_version END,
			num_cpu = CASE WHEN ? > 0 THEN ? ELSE num_cpu END,
			last_seen_at = ?, status = ?, consecutive_misses = 0, updated_at = ?
		WHERE id = ?`,
		hostname, hostname, osName, osName, arch, arch, agentVersion, agentVersion,
		numCPU, numCPU,
		formatTime(at), string(models.HostOnline), formatTime(at), id,
	)
	return err
}

func (s *Store) UpdateHostSnapshot(id string, osVersion, kernel string, reboot bool, disks []models.HostDisk, services []models.HostServiceStatus, sec *models.HostSecurity) error {
	_, err := s.db.Exec(`
		UPDATE hosts SET
			os_version = CASE WHEN ? != '' THEN ? ELSE os_version END,
			kernel_version = CASE WHEN ? != '' THEN ? ELSE kernel_version END,
			reboot_required = ?,
			disks_latest_json = ?, services_latest_json = ?, security_json = ?,
			updated_at = ?
		WHERE id = ?`,
		osVersion, osVersion, kernel, kernel, boolToInt(reboot),
		marshalJSON(disks), marshalJSON(services), marshalJSON(sec),
		formatTime(time.Now().UTC()), id,
	)
	return err
}

func (s *Store) UpdateHostOfflineState(id string, status models.HostStatus, misses int, at time.Time) error {
	_, err := s.db.Exec(`
		UPDATE hosts SET status = ?, consecutive_misses = ?, updated_at = ? WHERE id = ?`,
		string(status), misses, formatTime(at), id)
	return err
}

func (s *Store) DeleteHost(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM incidents WHERE monitor_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM hosts WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateHostEnrollToken(hostID, token string, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO host_enroll_tokens (id, host_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		newID(), hostID, hashAPIToken(token), formatTime(expiresAt), formatTime(time.Now().UTC()),
	)
	return err
}

func (s *Store) InvalidateUnusedHostEnrollTokens(hostID string) error {
	_, err := s.db.Exec(`
		UPDATE host_enroll_tokens SET used_at = ?
		WHERE host_id = ? AND used_at IS NULL`,
		formatTime(time.Now().UTC()), hostID)
	return err
}

func (s *Store) ConsumeHostEnrollToken(token string) (*models.Host, error) {
	if token == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var tokenID, hostID, expiresAt string
	var usedAt sql.NullString
	err = tx.QueryRow(`
		SELECT id, host_id, expires_at, used_at FROM host_enroll_tokens WHERE token_hash = ?`,
		hashAPIToken(token),
	).Scan(&tokenID, &hostID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if usedAt.Valid && usedAt.String != "" {
		return nil, nil
	}
	exp, err := parseTime(expiresAt)
	if err != nil || !exp.After(now) {
		return nil, nil
	}
	if _, err := tx.Exec(`UPDATE host_enroll_tokens SET used_at = ? WHERE id = ?`, formatTime(now), tokenID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetHost(hostID)
}

func (s *Store) LookupHostEnrollToken(token string) (*models.Host, error) {
	if token == "" {
		return nil, nil
	}
	var hostID, expiresAt string
	var usedAt sql.NullString
	err := s.db.QueryRow(`
		SELECT host_id, expires_at, used_at FROM host_enroll_tokens WHERE token_hash = ?`,
		hashAPIToken(token),
	).Scan(&hostID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if usedAt.Valid && usedAt.String != "" {
		return nil, nil
	}
	exp, err := parseTime(expiresAt)
	if err != nil || !exp.After(time.Now().UTC()) {
		return nil, nil
	}
	return s.GetHost(hostID)
}

func (s *Store) InsertHostSample(sample *models.HostSample) error {
	if sample.ID == "" {
		sample.ID = newID()
	}
	disks := sample.Disks
	if disks == nil {
		disks = []models.HostDisk{}
	}
	raw, err := json.Marshal(disks)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO host_samples (
			id, host_id, cpu_percent, mem_percent, swap_percent, iowait_percent,
			load1, load5, load15, disk_percent, disks_json, num_cpu, collected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ID, sample.HostID, sample.CPUPercent, sample.MemPercent, sample.SwapPercent, sample.IOWaitPercent,
		sample.Load1, sample.Load5, sample.Load15, sample.DiskPercent,
		string(raw), sample.NumCPU, formatTime(sample.CollectedAt),
	)
	return err
}

func (s *Store) GetHostStats(hostID string, since time.Time) (*models.HostStats, error) {
	rows, err := s.db.Query(`
		SELECT cpu_percent, mem_percent, swap_percent, iowait_percent, load1, disk_percent, disks_json, num_cpu, collected_at
		FROM host_samples WHERE host_id = ? AND collected_at >= ?
		ORDER BY collected_at ASC`,
		hostID, formatTime(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []models.HostStatsPoint
	for rows.Next() {
		var cpu, mem, swap, iowait, load1, disk sql.NullFloat64
		var disksJSON sql.NullString
		var numCPU sql.NullInt64
		var collected string
		if err := rows.Scan(&cpu, &mem, &swap, &iowait, &load1, &disk, &disksJSON, &numCPU, &collected); err != nil {
			return nil, err
		}
		p := models.HostStatsPoint{Disks: unmarshalDisks(nullableString(disksJSON))}
		if t, err := parseTime(collected); err == nil {
			p.Timestamp = t
		}
		if cpu.Valid {
			v := cpu.Float64
			p.CPUPercent = &v
		}
		if mem.Valid {
			v := mem.Float64
			p.MemPercent = &v
		}
		if swap.Valid {
			v := swap.Float64
			p.SwapPercent = &v
		}
		if iowait.Valid {
			v := iowait.Float64
			p.IOWaitPercent = &v
		}
		if load1.Valid {
			v := load1.Float64
			p.Load1 = &v
		}
		if disk.Valid {
			v := disk.Float64
			p.DiskPercent = &v
		}
		if numCPU.Valid {
			p.NumCPU = int(numCPU.Int64)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	points = downsampleHostPoints(points, maxChartPoints)
	if points == nil {
		points = []models.HostStatsPoint{}
	}
	return &models.HostStats{HostID: hostID, Points: points}, nil
}

func downsampleHostPoints(points []models.HostStatsPoint, max int) []models.HostStatsPoint {
	n := len(points)
	if max <= 0 || n <= max {
		return points
	}
	out := make([]models.HostStatsPoint, 0, max)
	for i := 0; i < max; i++ {
		start := i * n / max
		end := (i + 1) * n / max
		if end <= start {
			continue
		}
		best := points[start]
		for _, p := range points[start:end] {
			if hostPointPeak(p) > hostPointPeak(best) {
				best = p
			}
		}
		out = append(out, best)
	}
	return out
}

func hostPointPeak(p models.HostStatsPoint) float64 {
	max := 0.0
	for _, v := range []*float64{p.CPUPercent, p.MemPercent, p.SwapPercent, p.IOWaitPercent, p.DiskPercent, p.Load1} {
		if v != nil && *v > max {
			max = *v
		}
	}
	return max
}

func (s *Store) GetHostAlertState(hostID, metric string) (*models.HostAlertState, error) {
	var st models.HostAlertState
	err := s.db.QueryRow(`
		SELECT host_id, metric, consecutive_high, consecutive_ok
		FROM host_alert_state WHERE host_id = ? AND metric = ?`,
		hostID, metric,
	).Scan(&st.HostID, &st.Metric, &st.ConsecutiveHigh, &st.ConsecutiveOK)
	if err == sql.ErrNoRows {
		return &models.HostAlertState{HostID: hostID, Metric: metric}, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) UpsertHostAlertState(st *models.HostAlertState) error {
	_, err := s.db.Exec(`
		INSERT INTO host_alert_state (host_id, metric, consecutive_high, consecutive_ok)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(host_id, metric) DO UPDATE SET
			consecutive_high = excluded.consecutive_high,
			consecutive_ok = excluded.consecutive_ok`,
		st.HostID, st.Metric, st.ConsecutiveHigh, st.ConsecutiveOK,
	)
	return err
}

func (s *Store) PruneOldHostSamples(before time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM host_samples WHERE collected_at < ?`, formatTime(before))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) LatestHostSampleNumCPU(hostID string) int {
	var n sql.NullInt64
	_ = s.db.QueryRow(`
		SELECT num_cpu FROM host_samples WHERE host_id = ? ORDER BY collected_at DESC LIMIT 1`,
		hostID,
	).Scan(&n)
	if !n.Valid {
		return 0
	}
	return int(n.Int64)
}

func marshalJSON(v any) string {
	if v == nil {
		switch v.(type) {
		case *models.HostSecurity:
			return "{}"
		default:
			return "[]"
		}
	}
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return "[]"
	}
	return string(raw)
}

func unmarshalStringSlice(s string) []string {
	var out []string
	if s == "" {
		return []string{}
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func unmarshalDisks(s string) []models.HostDisk {
	var out []models.HostDisk
	if s == "" {
		return []models.HostDisk{}
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil || out == nil {
		return []models.HostDisk{}
	}
	return out
}

func unmarshalServiceStatus(s string) []models.HostServiceStatus {
	var out []models.HostServiceStatus
	if s == "" {
		return []models.HostServiceStatus{}
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil || out == nil {
		return []models.HostServiceStatus{}
	}
	return out
}

func unmarshalSecurity(s string) *models.HostSecurity {
	if s == "" || s == "{}" {
		return nil
	}
	var out models.HostSecurity
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return &out
}
