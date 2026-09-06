package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const perfTargetColumns = `id, name, url, method, interval_seconds, timeout_ms, slow_threshold_ms,
follow_redirects, enabled, alert_emails, http_username, http_password, tenant_id, alert_after_slow, consecutive_slow,
last_status, last_checked_at, created_at, updated_at`

func (s *Store) ListPerformanceTargets() ([]models.PerformanceTargetListItem, error) {
	return s.listPerformanceTargetsQuery("")
}

func (s *Store) ListPerformanceTargetsByTenant(tenantID string) ([]models.PerformanceTargetListItem, error) {
	if tenantID == "" {
		return []models.PerformanceTargetListItem{}, nil
	}
	return s.listPerformanceTargetsQuery(tenantID)
}

func (s *Store) listPerformanceTargetsQuery(tenantID string) ([]models.PerformanceTargetListItem, error) {
	q := `
		SELECT ` + perfTargetColumns + `,
			(SELECT response_time_ms FROM performance_results pr
			 WHERE pr.target_id = performance_targets.id ORDER BY checked_at DESC LIMIT 1) AS latest_rt
		FROM performance_targets`
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

	var items []models.PerformanceTargetListItem
	for rows.Next() {
		t, latest, err := scanPerformanceTargetRow(rows, true)
		if err != nil {
			return nil, err
		}
		item := models.PerformanceTargetListItem{PerformanceTarget: *t}
		if latest.Valid {
			v := int(latest.Int64)
			item.LatestResponseTimeMs = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEnabledPerformanceTargets() ([]models.PerformanceTarget, error) {
	rows, err := s.db.Query(`
		SELECT ` + perfTargetColumns + ` FROM performance_targets WHERE enabled = 1 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.PerformanceTarget
	for rows.Next() {
		t, _, err := scanPerformanceTargetRow(rows, false)
		if err != nil {
			return nil, err
		}
		targets = append(targets, *t)
	}
	return targets, rows.Err()
}

func (s *Store) GetPerformanceTarget(id string) (*models.PerformanceTarget, error) {
	row := s.db.QueryRow(`SELECT `+perfTargetColumns+` FROM performance_targets WHERE id = ?`, id)
	t, _, err := scanPerformanceTargetRow(row, false)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (s *Store) CreatePerformanceTarget(t *models.PerformanceTarget) error {
	now := time.Now().UTC()
	t.ID = newID()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Method == "" {
		t.Method = "GET"
	}
	if t.IntervalSeconds < 30 {
		t.IntervalSeconds = 300
	}
	if t.TimeoutMs == 0 {
		t.TimeoutMs = 10000
	}
	if t.SlowThresholdMs == 0 {
		t.SlowThresholdMs = 3000
	}
	if t.AlertAfterSlow < 1 {
		t.AlertAfterSlow = 2
	}
	t.Enabled = true
	t.LastStatus = models.StatusUnknown

	_, err := s.db.Exec(`
		INSERT INTO performance_targets (`+perfTargetColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.URL, t.Method, t.IntervalSeconds, t.TimeoutMs, t.SlowThresholdMs,
		boolToInt(t.FollowRedirects), boolToInt(t.Enabled), t.AlertEmails,
		t.HTTPUsername, t.HTTPPassword, nullString(t.TenantID),
		t.AlertAfterSlow, t.ConsecutiveSlow,
		string(t.LastStatus), nil, formatTime(t.CreatedAt), formatTime(t.UpdatedAt),
	)
	return err
}

func (s *Store) UpdatePerformanceTarget(t *models.PerformanceTarget) error {
	t.UpdatedAt = time.Now().UTC()
	var lastChecked any
	if t.LastCheckedAt != nil {
		lastChecked = formatTime(*t.LastCheckedAt)
	}
	if t.AlertAfterSlow < 1 {
		t.AlertAfterSlow = 2
	}
	if t.IntervalSeconds < 30 {
		t.IntervalSeconds = 30
	}
	_, err := s.db.Exec(`
		UPDATE performance_targets SET
			name=?, url=?, method=?, interval_seconds=?, timeout_ms=?, slow_threshold_ms=?,
			follow_redirects=?, enabled=?, alert_emails=?, http_username=?, http_password=?,
			tenant_id=?, alert_after_slow=?, last_status=?, last_checked_at=?, updated_at=?
		WHERE id=?`,
		t.Name, t.URL, t.Method, t.IntervalSeconds, t.TimeoutMs, t.SlowThresholdMs,
		boolToInt(t.FollowRedirects), boolToInt(t.Enabled), t.AlertEmails,
		t.HTTPUsername, t.HTTPPassword, nullString(t.TenantID),
		t.AlertAfterSlow, string(t.LastStatus), lastChecked, formatTime(t.UpdatedAt), t.ID,
	)
	return err
}

func (s *Store) DeletePerformanceTarget(id string) error {
	if _, err := s.db.Exec(`DELETE FROM incidents WHERE monitor_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM performance_targets WHERE id = ?`, id)
	return err
}

func (s *Store) SetPerformanceTargetEnabled(id string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE performance_targets SET enabled=?, updated_at=? WHERE id=?`,
		boolToInt(enabled), formatTime(time.Now().UTC()), id,
	)
	return err
}

func (s *Store) UpdatePerformanceTargetAfterCheck(id string, status models.MonitorStatus, consecutiveSlow int, checkedAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE performance_targets SET last_status=?, consecutive_slow=?, last_checked_at=?, updated_at=? WHERE id=?`,
		string(status), consecutiveSlow, formatTime(checkedAt), formatTime(time.Now().UTC()), id,
	)
	return err
}

func scanPerformanceTargetRow(row interface {
	Scan(dest ...any) error
}, withLatest bool) (*models.PerformanceTarget, sql.NullInt64, error) {
	var t models.PerformanceTarget
	var lastStatus string
	var followRedirects, enabled int
	var alertEmails, httpUser, httpPass, tenantID, lastCheckedAt, createdAt, updatedAt sql.NullString
	var latestRT sql.NullInt64

	dest := []any{
		&t.ID, &t.Name, &t.URL, &t.Method, &t.IntervalSeconds, &t.TimeoutMs, &t.SlowThresholdMs,
		&followRedirects, &enabled, &alertEmails, &httpUser, &httpPass, &tenantID, &t.AlertAfterSlow, &t.ConsecutiveSlow,
		&lastStatus, &lastCheckedAt, &createdAt, &updatedAt,
	}
	if withLatest {
		dest = append(dest, &latestRT)
	}

	if err := row.Scan(dest...); err != nil {
		return nil, latestRT, err
	}

	t.FollowRedirects = intToBool(followRedirects)
	t.Enabled = intToBool(enabled)
	t.AlertEmails = nullableString(alertEmails)
	t.HTTPUsername = nullableString(httpUser)
	t.HTTPPassword = nullableString(httpPass)
	t.TenantID = nullableString(tenantID)
	if t.AlertAfterSlow < 1 {
		t.AlertAfterSlow = 2
	}
	t.LastStatus = models.MonitorStatus(lastStatus)
	t.LastCheckedAt = nullableTime(lastCheckedAt)
	if ct, err := parseTime(nullableString(createdAt)); err == nil {
		t.CreatedAt = ct
	}
	if ut, err := parseTime(nullableString(updatedAt)); err == nil {
		t.UpdatedAt = ut
	}
	return &t, latestRT, nil
}

// HTTPAuthForURL returns basic-auth credentials from an HTTP monitor with the
// same tenant and URL. Used when a performance target is created from the
// monitor form (the API never returns the stored password).
func (s *Store) HTTPAuthForURL(tenantID, rawURL string) (username, password string, ok bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", false
	}
	var user, pass sql.NullString
	err := s.db.QueryRow(`
		SELECT http_username, http_password FROM monitors
		WHERE type = 'http' AND url = ? AND COALESCE(tenant_id, '') = ? AND http_username != ''
		ORDER BY updated_at DESC LIMIT 1`,
		rawURL, strings.TrimSpace(tenantID),
	).Scan(&user, &pass)
	if err != nil || strings.TrimSpace(user.String) == "" {
		return "", "", false
	}
	return user.String, pass.String, true
}

func (s *Store) FillPerformanceHTTPAuth(t *models.PerformanceTarget) {
	if t == nil {
		return
	}
	if strings.TrimSpace(t.HTTPUsername) == "" || strings.TrimSpace(t.HTTPPassword) != "" {
		return
	}
	_, pass, ok := s.HTTPAuthForURL(t.TenantID, t.URL)
	if ok {
		t.HTTPPassword = pass
	}
}

// InheritPerformanceHTTPAuth copies username and password from a sibling HTTP
// monitor when the performance target is missing either value.
func (s *Store) InheritPerformanceHTTPAuth(t *models.PerformanceTarget) {
	if t == nil {
		return
	}
	user, pass, ok := s.HTTPAuthForURL(t.TenantID, t.URL)
	if !ok {
		return
	}
	if strings.TrimSpace(t.HTTPUsername) == "" {
		t.HTTPUsername = user
	}
	if strings.TrimSpace(t.HTTPPassword) == "" {
		t.HTTPPassword = pass
	}
}
