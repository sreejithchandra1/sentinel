package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const monitorColumns = `id, type, name, url, port, config, method, expected_status, expected_status_min, expected_status_max,
keyword_must_exist, keyword_must_not_exist, request_body, request_headers, http_username, http_password,
interval_seconds, timeout_ms, slow_threshold_ms, follow_redirects, alert_emails, enabled,
notify_email, notify_slack, notify_webhooks, invert,
tags, heartbeat_token, tenant_id, alert_after_failures,
consecutive_failures, last_status, last_checked_at, created_at, updated_at`

type monitorScanRow struct {
	id, monitorType, name, url, method, lastStatus                                                      string
	config, keywordExist, keywordNotExist, requestBody, requestHeaders, httpUser, httpPass, alertEmails sql.NullString
	tagsRaw, heartbeatToken, tenantID                                                                   sql.NullString
	port, expectedMin, expectedMax                                                                      sql.NullInt64
	followRedirects, enabled, notifyEmail, notifySlack, notifyWebhooks, invert                          int
	expectedStatus, intervalSeconds, timeoutMs                                                          int
	slowThresholdMs, alertAfterFailures, consecutiveFailures                                            int
	lastCheckedAt, createdAt, updatedAt                                                                 sql.NullString
	latestRT                                                                                            sql.NullInt64
}

func (r *monitorScanRow) scan(row interface{ Scan(dest ...any) error }) error {
	return row.Scan(
		&r.id, &r.monitorType, &r.name, &r.url, &r.port, &r.config, &r.method,
		&r.expectedStatus, &r.expectedMin, &r.expectedMax,
		&r.keywordExist, &r.keywordNotExist, &r.requestBody, &r.requestHeaders, &r.httpUser, &r.httpPass,
		&r.intervalSeconds, &r.timeoutMs, &r.slowThresholdMs,
		&r.followRedirects, &r.alertEmails, &r.enabled,
		&r.notifyEmail, &r.notifySlack, &r.notifyWebhooks, &r.invert,
		&r.tagsRaw, &r.heartbeatToken, &r.tenantID, &r.alertAfterFailures,
		&r.consecutiveFailures, &r.lastStatus, &r.lastCheckedAt,
		&r.createdAt, &r.updatedAt,
	)
}

func (r *monitorScanRow) scanWithLatestRT(row interface{ Scan(dest ...any) error }) error {
	return row.Scan(
		&r.id, &r.monitorType, &r.name, &r.url, &r.port, &r.config, &r.method,
		&r.expectedStatus, &r.expectedMin, &r.expectedMax,
		&r.keywordExist, &r.keywordNotExist, &r.requestBody, &r.requestHeaders, &r.httpUser, &r.httpPass,
		&r.intervalSeconds, &r.timeoutMs, &r.slowThresholdMs,
		&r.followRedirects, &r.alertEmails, &r.enabled,
		&r.notifyEmail, &r.notifySlack, &r.notifyWebhooks, &r.invert,
		&r.tagsRaw, &r.heartbeatToken, &r.tenantID, &r.alertAfterFailures,
		&r.consecutiveFailures, &r.lastStatus, &r.lastCheckedAt,
		&r.createdAt, &r.updatedAt, &r.latestRT,
	)
}

func (r *monitorScanRow) toMonitor() models.Monitor {
	m := models.Monitor{
		ID:                  r.id,
		Type:                models.MonitorType(r.monitorType),
		Name:                r.name,
		URL:                 r.url,
		Port:                nullableInt(r.port),
		Config:              nullableString(r.config),
		Method:              r.method,
		ExpectedStatus:      r.expectedStatus,
		ExpectedStatusMin:   nullableInt(r.expectedMin),
		ExpectedStatusMax:   nullableInt(r.expectedMax),
		KeywordMustExist:    nullableString(r.keywordExist),
		KeywordMustNotExist: nullableString(r.keywordNotExist),
		RequestBody:         nullableString(r.requestBody),
		RequestHeaders:      nullableString(r.requestHeaders),
		HTTPUsername:        nullableString(r.httpUser),
		HTTPPassword:        nullableString(r.httpPass),
		HTTPAuthSet:         nullableString(r.httpUser) != "" || nullableString(r.httpPass) != "",
		IntervalSeconds:     r.intervalSeconds,
		TimeoutMs:           r.timeoutMs,
		SlowThresholdMs:     r.slowThresholdMs,
		FollowRedirects:     intToBool(r.followRedirects),
		AlertEmails:         nullableString(r.alertEmails),
		Enabled:             intToBool(r.enabled),
		NotifyEmail:         intToBool(r.notifyEmail),
		NotifySlack:         intToBool(r.notifySlack),
		NotifyWebhooks:      intToBool(r.notifyWebhooks),
		Invert:              intToBool(r.invert),
		Tags:                decodeTags(nullableString(r.tagsRaw)),
		HeartbeatToken:      nullableString(r.heartbeatToken),
		TenantID:            nullableString(r.tenantID),
		AlertAfterFailures:  r.alertAfterFailures,
		ConsecutiveFailures: r.consecutiveFailures,
		LastStatus:          models.MonitorStatus(r.lastStatus),
		LastCheckedAt:       nullableTime(r.lastCheckedAt),
	}
	if m.Type == "" {
		m.Type = models.MonitorHTTP
	}
	if m.AlertAfterFailures < 1 {
		m.AlertAfterFailures = 2
	}
	if t, err := parseTime(nullableString(r.createdAt)); err == nil {
		m.CreatedAt = t
	}
	if t, err := parseTime(nullableString(r.updatedAt)); err == nil {
		m.UpdatedAt = t
	}
	return m
}

func (s *Store) scanMonitorRow(row interface {
	Scan(dest ...any) error
}) (*models.Monitor, error) {
	var r monitorScanRow
	if err := r.scan(row); err != nil {
		return nil, err
	}
	m := r.toMonitor()
	return &m, nil
}

func (s *Store) ListMonitors() ([]models.MonitorListItem, error) {
	return s.listMonitorsQuery("")
}

func (s *Store) ListMonitorsByTenant(tenantID string) ([]models.MonitorListItem, error) {
	if tenantID == "" {
		return []models.MonitorListItem{}, nil
	}
	return s.listMonitorsQuery(tenantID)
}

func (s *Store) listMonitorsQuery(tenantID string) ([]models.MonitorListItem, error) {
	q := `
		SELECT ` + monitorColumns + `,
			(SELECT response_time_ms FROM check_results cr WHERE cr.monitor_id = monitors.id ORDER BY checked_at DESC LIMIT 1) AS latest_rt
		FROM monitors`
	var args []any
	if tenantID != "" {
		q += ` WHERE tenant_id = ?`
		args = append(args, tenantID)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MonitorListItem
	for rows.Next() {
		var r monitorScanRow
		if err := r.scanWithLatestRT(rows); err != nil {
			return nil, err
		}
		item := models.MonitorListItem{Monitor: r.toMonitor()}
		if r.latestRT.Valid {
			v := int(r.latestRT.Int64)
			item.LatestResponseTimeMs = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListMonitorTenants(ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT id, COALESCE(tenant_id, '') FROM monitors WHERE id IN (`+sqlPlaceholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, tenantID string
		if err := rows.Scan(&id, &tenantID); err != nil {
			return nil, err
		}
		out[id] = tenantID
	}
	return out, rows.Err()
}

func (s *Store) GetMonitor(id string) (*models.Monitor, error) {
	row := s.db.QueryRow(`SELECT `+monitorColumns+` FROM monitors WHERE id = ?`, id)
	m, err := s.scanMonitorRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (s *Store) CreateMonitor(m *models.Monitor) error {
	now := time.Now().UTC()
	m.ID = newID()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Type == "" {
		m.Type = models.MonitorHTTP
	}
	if m.Method == "" {
		m.Method = "GET"
	}
	if m.IntervalSeconds < 30 {
		m.IntervalSeconds = 60
	}
	if m.TimeoutMs == 0 {
		m.TimeoutMs = 10000
	}
	if m.SlowThresholdMs == 0 {
		m.SlowThresholdMs = 3000
	}
	if m.ExpectedStatus == 0 {
		m.ExpectedStatus = 200
	}
	if m.AlertAfterFailures < 1 {
		m.AlertAfterFailures = 2
	}
	m.LastStatus = models.StatusUnknown

	_, err := s.db.Exec(`
		INSERT INTO monitors (`+monitorColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, string(m.Type), m.Name, m.URL, m.Port, nullString(m.Config), m.Method,
		m.ExpectedStatus, m.ExpectedStatusMin, m.ExpectedStatusMax,
		nullString(m.KeywordMustExist), nullString(m.KeywordMustNotExist),
		nullString(m.RequestBody), nullString(m.RequestHeaders),
		m.HTTPUsername, m.HTTPPassword,
		m.IntervalSeconds, m.TimeoutMs, m.SlowThresholdMs,
		boolToInt(m.FollowRedirects), nullString(m.AlertEmails), boolToInt(m.Enabled),
		boolToInt(m.NotifyEmail), boolToInt(m.NotifySlack), boolToInt(m.NotifyWebhooks), boolToInt(m.Invert),
		encodeTags(m.Tags), nullString(m.HeartbeatToken), nullString(m.TenantID), m.AlertAfterFailures,
		m.ConsecutiveFailures, string(m.LastStatus), nil,
		formatTime(m.CreatedAt), formatTime(m.UpdatedAt),
	)
	return err
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) UpdateMonitor(m *models.Monitor) error {
	m.UpdatedAt = time.Now().UTC()
	var lastChecked interface{}
	if m.LastCheckedAt != nil {
		lastChecked = formatTime(*m.LastCheckedAt)
	}
	if m.Type == "" {
		m.Type = models.MonitorHTTP
	}
	if m.AlertAfterFailures < 1 {
		m.AlertAfterFailures = 2
	}
	if m.IntervalSeconds < 30 {
		m.IntervalSeconds = 30
	}

	res, err := s.db.Exec(`
		UPDATE monitors SET
			type=?, name=?, url=?, port=?, config=?, method=?, expected_status=?, expected_status_min=?, expected_status_max=?,
			keyword_must_exist=?, keyword_must_not_exist=?, request_body=?, request_headers=?, http_username=?, http_password=?,
			interval_seconds=?, timeout_ms=?, slow_threshold_ms=?, follow_redirects=?,
			alert_emails=?, enabled=?, notify_email=?, notify_slack=?, notify_webhooks=?, invert=?, tags=?, heartbeat_token=?, tenant_id=?, alert_after_failures=?,
			consecutive_failures=?, last_status=?, last_checked_at=?,
			updated_at=?
		WHERE id=?`,
		string(m.Type), m.Name, m.URL, m.Port, nullString(m.Config), m.Method,
		m.ExpectedStatus, m.ExpectedStatusMin, m.ExpectedStatusMax,
		nullString(m.KeywordMustExist), nullString(m.KeywordMustNotExist),
		nullString(m.RequestBody), nullString(m.RequestHeaders),
		m.HTTPUsername, m.HTTPPassword,
		m.IntervalSeconds, m.TimeoutMs, m.SlowThresholdMs, boolToInt(m.FollowRedirects),
		nullString(m.AlertEmails), boolToInt(m.Enabled),
		boolToInt(m.NotifyEmail), boolToInt(m.NotifySlack), boolToInt(m.NotifyWebhooks), boolToInt(m.Invert),
		encodeTags(m.Tags), nullString(m.HeartbeatToken), nullString(m.TenantID), m.AlertAfterFailures,
		m.ConsecutiveFailures, string(m.LastStatus), lastChecked,
		formatTime(m.UpdatedAt), m.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("monitor not found")
	}
	return nil
}

func (s *Store) DeleteMonitor(id string) error {
	if _, err := s.db.Exec(`DELETE FROM incidents WHERE monitor_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM monitors WHERE id = ?`, id)
	return err
}

func (s *Store) SetMonitorEnabled(id string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE monitors SET enabled=?, updated_at=? WHERE id=?`,
		boolToInt(enabled), formatTime(time.Now().UTC()), id,
	)
	return err
}

func (s *Store) ListEnabledMonitors() ([]models.Monitor, error) {
	rows, err := s.db.Query(`SELECT ` + monitorColumns + ` FROM monitors WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []models.Monitor
	for rows.Next() {
		m, err := s.scanMonitorRow(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, *m)
	}
	return monitors, rows.Err()
}

func (s *Store) UpdateMonitorState(id string, status models.MonitorStatus, consecutiveFailures int, checkedAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE monitors SET last_status=?, consecutive_failures=?, last_checked_at=?, updated_at=?
		WHERE id=?`,
		string(status), consecutiveFailures, formatTime(checkedAt), formatTime(time.Now().UTC()), id,
	)
	return err
}
