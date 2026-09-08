package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const hostColumns = `id, name, hostname, tenant_id, os, arch, agent_version, last_seen_at, status,
enabled, interval_seconds, alert_after_failures, consecutive_misses,
collect_cpu, collect_memory, collect_disk, collect_load,
alert_cpu_enabled, alert_cpu_threshold, alert_cpu_after,
alert_memory_enabled, alert_memory_threshold, alert_memory_after,
alert_disk_enabled, alert_disk_threshold, alert_disk_after,
alert_load_enabled, alert_load_threshold, alert_load_after,
alert_emails, notify_email, notify_slack, notify_webhooks,
created_at, updated_at`

type hostScanRow struct {
	id, name, hostname, tenantID, os, arch, agentVersion, status     string
	lastSeenAt, createdAt, updatedAt                                 sql.NullString
	enabled, collectCPU, collectMemory, collectDisk, collectLoad     int
	notifyEmail, notifySlack, notifyWebhooks                         int
	alertCPU, alertMem, alertDisk, alertLoad                         int
	intervalSeconds, alertAfter, consecutiveMisses                   int
	alertCPUAfter, alertMemAfter, alertDiskAfter, alertLoadAfter     int
	alertCPUThresh, alertMemThresh, alertDiskThresh, alertLoadThresh float64
	alertEmails                                                      string
}

func (r *hostScanRow) scan(row interface{ Scan(dest ...any) error }) error {
	return row.Scan(
		&r.id, &r.name, &r.hostname, &r.tenantID, &r.os, &r.arch, &r.agentVersion, &r.lastSeenAt, &r.status,
		&r.enabled, &r.intervalSeconds, &r.alertAfter, &r.consecutiveMisses,
		&r.collectCPU, &r.collectMemory, &r.collectDisk, &r.collectLoad,
		&r.alertCPU, &r.alertCPUThresh, &r.alertCPUAfter,
		&r.alertMem, &r.alertMemThresh, &r.alertMemAfter,
		&r.alertDisk, &r.alertDiskThresh, &r.alertDiskAfter,
		&r.alertLoad, &r.alertLoadThresh, &r.alertLoadAfter,
		&r.alertEmails, &r.notifyEmail, &r.notifySlack, &r.notifyWebhooks,
		&r.createdAt, &r.updatedAt,
	)
}

func (r *hostScanRow) toHost() models.Host {
	h := models.Host{
		ID:                   r.id,
		Name:                 r.name,
		Hostname:             r.hostname,
		TenantID:             r.tenantID,
		OS:                   r.os,
		Arch:                 r.arch,
		AgentVersion:         r.agentVersion,
		LastSeenAt:           nullableTime(r.lastSeenAt),
		Status:               models.HostStatus(r.status),
		Enabled:              intToBool(r.enabled),
		IntervalSeconds:      r.intervalSeconds,
		AlertAfterFailures:   r.alertAfter,
		ConsecutiveMisses:    r.consecutiveMisses,
		CollectCPU:           intToBool(r.collectCPU),
		CollectMemory:        intToBool(r.collectMemory),
		CollectDisk:          intToBool(r.collectDisk),
		CollectLoad:          intToBool(r.collectLoad),
		AlertCPUEnabled:      intToBool(r.alertCPU),
		AlertCPUThreshold:    r.alertCPUThresh,
		AlertCPUAfter:        r.alertCPUAfter,
		AlertMemoryEnabled:   intToBool(r.alertMem),
		AlertMemoryThreshold: r.alertMemThresh,
		AlertMemoryAfter:     r.alertMemAfter,
		AlertDiskEnabled:     intToBool(r.alertDisk),
		AlertDiskThreshold:   r.alertDiskThresh,
		AlertDiskAfter:       r.alertDiskAfter,
		AlertLoadEnabled:     intToBool(r.alertLoad),
		AlertLoadThreshold:   r.alertLoadThresh,
		AlertLoadAfter:       r.alertLoadAfter,
		AlertEmails:          r.alertEmails,
		NotifyEmail:          intToBool(r.notifyEmail),
		NotifySlack:          intToBool(r.notifySlack),
		NotifyWebhooks:       intToBool(r.notifyWebhooks),
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
	if !h.CollectCPU && !h.CollectMemory && !h.CollectDisk && !h.CollectLoad {
		h.CollectCPU, h.CollectMemory, h.CollectDisk, h.CollectLoad = true, true, true, true
	}
	_, err := s.db.Exec(`
		INSERT INTO hosts (
			id, name, hostname, tenant_id, os, arch, agent_version, token_hash, last_seen_at, status,
			enabled, interval_seconds, alert_after_failures, consecutive_misses,
			collect_cpu, collect_memory, collect_disk, collect_load,
			alert_cpu_enabled, alert_cpu_threshold, alert_cpu_after,
			alert_memory_enabled, alert_memory_threshold, alert_memory_after,
			alert_disk_enabled, alert_disk_threshold, alert_disk_after,
			alert_load_enabled, alert_load_threshold, alert_load_after,
			alert_emails, notify_email, notify_slack, notify_webhooks,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.Hostname, h.TenantID, h.OS, h.Arch, h.AgentVersion,
		nullableTimeArg(h.LastSeenAt), string(h.Status),
		boolToInt(h.Enabled), h.IntervalSeconds, h.AlertAfterFailures, h.ConsecutiveMisses,
		boolToInt(h.CollectCPU), boolToInt(h.CollectMemory), boolToInt(h.CollectDisk), boolToInt(h.CollectLoad),
		boolToInt(h.AlertCPUEnabled), h.AlertCPUThreshold, h.AlertCPUAfter,
		boolToInt(h.AlertMemoryEnabled), h.AlertMemoryThreshold, h.AlertMemoryAfter,
		boolToInt(h.AlertDiskEnabled), h.AlertDiskThreshold, h.AlertDiskAfter,
		boolToInt(h.AlertLoadEnabled), h.AlertLoadThreshold, h.AlertLoadAfter,
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
			name = ?, hostname = ?, tenant_id = ?, os = ?, arch = ?, agent_version = ?,
			last_seen_at = ?, status = ?, enabled = ?, interval_seconds = ?,
			alert_after_failures = ?, consecutive_misses = ?,
			collect_cpu = ?, collect_memory = ?, collect_disk = ?, collect_load = ?,
			alert_cpu_enabled = ?, alert_cpu_threshold = ?, alert_cpu_after = ?,
			alert_memory_enabled = ?, alert_memory_threshold = ?, alert_memory_after = ?,
			alert_disk_enabled = ?, alert_disk_threshold = ?, alert_disk_after = ?,
			alert_load_enabled = ?, alert_load_threshold = ?, alert_load_after = ?,
			alert_emails = ?, notify_email = ?, notify_slack = ?, notify_webhooks = ?,
			updated_at = ?
		WHERE id = ?`,
		h.Name, h.Hostname, h.TenantID, h.OS, h.Arch, h.AgentVersion,
		nullableTimeArg(h.LastSeenAt), string(h.Status), boolToInt(h.Enabled), h.IntervalSeconds,
		h.AlertAfterFailures, h.ConsecutiveMisses,
		boolToInt(h.CollectCPU), boolToInt(h.CollectMemory), boolToInt(h.CollectDisk), boolToInt(h.CollectLoad),
		boolToInt(h.AlertCPUEnabled), h.AlertCPUThreshold, h.AlertCPUAfter,
		boolToInt(h.AlertMemoryEnabled), h.AlertMemoryThreshold, h.AlertMemoryAfter,
		boolToInt(h.AlertDiskEnabled), h.AlertDiskThreshold, h.AlertDiskAfter,
		boolToInt(h.AlertLoadEnabled), h.AlertLoadThreshold, h.AlertLoadAfter,
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
			last_seen_at = ?, status = ?, consecutive_misses = 0, updated_at = ?
		WHERE id = ?`,
		hostname, hostname, osName, osName, arch, arch, agentVersion, agentVersion,
		formatTime(at), string(models.HostOnline), formatTime(at), id,
	)
	_ = numCPU
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
			id, host_id, cpu_percent, mem_percent, load1, load5, load15,
			disk_percent, disks_json, num_cpu, collected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ID, sample.HostID, sample.CPUPercent, sample.MemPercent,
		sample.Load1, sample.Load5, sample.Load15, sample.DiskPercent,
		string(raw), sample.NumCPU, formatTime(sample.CollectedAt),
	)
	return err
}

func (s *Store) GetHostStats(hostID string, since time.Time) (*models.HostStats, error) {
	rows, err := s.db.Query(`
		SELECT cpu_percent, mem_percent, load1, disk_percent, collected_at
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
		var cpu, mem, load1, disk sql.NullFloat64
		var collected string
		if err := rows.Scan(&cpu, &mem, &load1, &disk, &collected); err != nil {
			return nil, err
		}
		p := models.HostStatsPoint{}
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
		if load1.Valid {
			v := load1.Float64
			p.Load1 = &v
		}
		if disk.Valid {
			v := disk.Float64
			p.DiskPercent = &v
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
	if p.CPUPercent != nil && *p.CPUPercent > max {
		max = *p.CPUPercent
	}
	if p.MemPercent != nil && *p.MemPercent > max {
		max = *p.MemPercent
	}
	if p.DiskPercent != nil && *p.DiskPercent > max {
		max = *p.DiskPercent
	}
	if p.Load1 != nil && *p.Load1 > max {
		max = *p.Load1
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
