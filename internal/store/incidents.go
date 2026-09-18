package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

var ErrIncidentResolved = errors.New("incident already resolved")

func (s *Store) CreateIncident(inc *models.Incident) error {
	inc.ID = newID()
	var resolved any
	if inc.ResolvedAt != nil {
		resolved = formatTime(*inc.ResolvedAt)
	}
	details := incidentDetailsJSON(inc)
	inc.Details = details
	inc.Message = models.StripHTTPErrorSourceSuffix(inc.Message)
	_, err := s.db.Exec(`
		INSERT INTO incidents (id, monitor_id, type, message, started_at, resolved_at, details)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		inc.ID, inc.MonitorID, string(inc.Type), inc.Message,
		formatTime(inc.StartedAt), resolved, details,
	)
	return err
}

func incidentDetailsJSON(inc *models.Incident) string {
	if inc == nil {
		return ""
	}
	if inc.ErrorPage != nil {
		if inc.ErrorPage.ViewToken == "" {
			inc.ErrorPage.ViewToken = strings.ReplaceAll(newID(), "-", "")
		}
		b, err := json.Marshal(inc.ErrorPage)
		if err == nil {
			return string(b)
		}
	}
	return inc.Details
}

func decodeIncidentErrorPage(details string) *models.HTTPErrorPage {
	details = strings.TrimSpace(details)
	if details == "" {
		return nil
	}
	var page models.HTTPErrorPage
	if err := json.Unmarshal([]byte(details), &page); err != nil {
		return nil
	}
	if page.StatusCode == 0 && page.Source == "" && page.BodyHTML == "" && page.ViewToken == "" {
		return nil
	}
	return &page
}

func (s *Store) ResolveOpenIncidents(monitorID string, incidentType models.IncidentType, resolvedAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE incidents SET resolved_at = ?
		WHERE monitor_id = ? AND type = ? AND resolved_at IS NULL`,
		formatTime(resolvedAt), monitorID, string(incidentType),
	)
	return err
}

func (s *Store) HasOpenIncident(monitorID string, incidentType models.IncidentType) (bool, error) {
	inc, err := s.GetOpenIncident(monitorID, incidentType)
	if err != nil {
		return false, err
	}
	return inc != nil, nil
}

func (s *Store) ResolveIncident(id string, resolvedAt time.Time) error {
	_, err := s.db.Exec(`UPDATE incidents SET resolved_at = ? WHERE id = ?`, formatTime(resolvedAt), id)
	return err
}

func (s *Store) GetOpenIncident(monitorID string, incidentType models.IncidentType) (*models.Incident, error) {
	row := s.db.QueryRow(`
		SELECT id, monitor_id, type, message, started_at, resolved_at, acknowledged_at, acknowledged_by
		FROM incidents
		WHERE monitor_id = ? AND type = ? AND resolved_at IS NULL
		ORDER BY started_at DESC LIMIT 1`,
		monitorID, string(incidentType),
	)
	return scanIncident(row)
}

func scanIncident(row interface {
	Scan(dest ...any) error
}) (*models.Incident, error) {
	var inc models.Incident
	var incType string
	var startedAt string
	var resolvedAt, ackedAt, ackedBy sql.NullString

	err := row.Scan(&inc.ID, &inc.MonitorID, &incType, &inc.Message, &startedAt, &resolvedAt, &ackedAt, &ackedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	inc.Type = models.IncidentType(incType)
	inc.Message = models.StripHTTPErrorSourceSuffix(inc.Message)
	applyIncidentTimes(&inc, startedAt, resolvedAt, ackedAt, ackedBy)
	return &inc, nil
}

func applyIncidentTimes(inc *models.Incident, startedAt string, resolvedAt, ackedAt, ackedBy sql.NullString) {
	if t, err := parseTime(startedAt); err == nil {
		inc.StartedAt = t
	}
	if resolvedAt.Valid && resolvedAt.String != "" {
		if t, err := parseTime(resolvedAt.String); err == nil {
			inc.ResolvedAt = &t
		}
	}
	if ackedAt.Valid && ackedAt.String != "" {
		if t, err := parseTime(ackedAt.String); err == nil {
			inc.AcknowledgedAt = &t
		}
	}
	if ackedBy.Valid {
		inc.AcknowledgedBy = ackedBy.String
	}
}

func (s *Store) GetIncident(id string) (*models.IncidentListItem, string, error) {
	row := s.db.QueryRow(`
		SELECT i.id, i.monitor_id, i.type, i.message, i.started_at, i.resolved_at,
			i.acknowledged_at, i.acknowledged_by, i.details,
			COALESCE(m.name, pt.name, h.name, h.hostname, '') AS monitor_name,
			COALESCE(m.tenant_id, pt.tenant_id, h.tenant_id, '') AS tenant_id
		FROM incidents i
		LEFT JOIN monitors m ON m.id = i.monitor_id
		LEFT JOIN performance_targets pt ON pt.id = i.monitor_id
		LEFT JOIN hosts h ON h.id = i.monitor_id
		WHERE i.id = ?`, id)

	var item models.IncidentListItem
	var incType string
	var startedAt string
	var resolvedAt, ackedAt, ackedBy, details sql.NullString
	var tenantID string
	err := row.Scan(
		&item.ID, &item.MonitorID, &incType, &item.Message, &startedAt, &resolvedAt,
		&ackedAt, &ackedBy, &details, &item.MonitorName, &tenantID,
	)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	item.Type = models.IncidentType(incType)
	item.Message = models.StripHTTPErrorSourceSuffix(item.Message)
	applyIncidentTimes(&item.Incident, startedAt, resolvedAt, ackedAt, ackedBy)
	if details.Valid {
		item.Details = details.String
		item.ErrorPage = decodeIncidentErrorPage(details.String)
	}
	return &item, tenantID, nil
}

func (s *Store) AcknowledgeIncident(id, by string, at time.Time) (*models.IncidentListItem, error) {
	item, _, err := s.GetIncident(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	if item.ResolvedAt != nil {
		return item, ErrIncidentResolved
	}
	if item.AcknowledgedAt != nil {
		return item, nil
	}
	_, err = s.db.Exec(`
		UPDATE incidents SET acknowledged_at = ?, acknowledged_by = ?
		WHERE id = ? AND resolved_at IS NULL AND acknowledged_at IS NULL`,
		formatTime(at), by, id,
	)
	if err != nil {
		return nil, err
	}
	item, _, err = s.GetIncident(id)
	if err != nil {
		return nil, err
	}
	if item != nil && item.AcknowledgedAt == nil && item.ResolvedAt != nil {
		return item, ErrIncidentResolved
	}
	return item, nil
}

// IncidentQuery filters incident listings.
type IncidentQuery struct {
	OpenOnly  bool
	Status    string // "", "open", "resolved"
	Type      string
	TenantID  string
	MonitorID string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

func (s *Store) ListIncidents(limit int, openOnly bool) ([]models.IncidentListItem, error) {
	return s.QueryIncidents(IncidentQuery{Limit: limit, OpenOnly: openOnly})
}

func (s *Store) ListIncidentsByTenant(limit int, openOnly bool, tenantID string) ([]models.IncidentListItem, error) {
	if tenantID == "" {
		return []models.IncidentListItem{}, nil
	}
	return s.QueryIncidents(IncidentQuery{Limit: limit, OpenOnly: openOnly, TenantID: tenantID})
}

func (s *Store) ListIncidentsScoped(limit int, openOnly bool, tenantID string) ([]models.IncidentListItem, error) {
	return s.QueryIncidents(IncidentQuery{Limit: limit, OpenOnly: openOnly, TenantID: tenantID})
}

func (s *Store) ListIncidentsByMonitor(monitorID string, limit, offset int) ([]models.IncidentListItem, error) {
	return s.QueryIncidents(IncidentQuery{MonitorID: monitorID, Limit: limit, Offset: offset})
}

func (s *Store) CountIncidentsByMonitor(monitorID string) (int, error) {
	return s.CountIncidents(IncidentQuery{MonitorID: monitorID})
}

func (s *Store) QueryIncidents(q IncidentQuery) ([]models.IncidentListItem, error) {
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	sqlQ := `
		SELECT i.id, i.monitor_id, i.type, i.message, i.started_at, i.resolved_at,
			i.acknowledged_at, i.acknowledged_by,
			COALESCE(m.name, pt.name, h.name, h.hostname, '') AS monitor_name
		FROM incidents i
		LEFT JOIN monitors m ON m.id = i.monitor_id
		LEFT JOIN performance_targets pt ON pt.id = i.monitor_id
		LEFT JOIN hosts h ON h.id = i.monitor_id`
	conds, args := incidentConds(q)
	if len(conds) > 0 {
		sqlQ += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	sqlQ += ` ORDER BY CASE WHEN i.resolved_at IS NULL THEN 0 ELSE 1 END, i.started_at DESC LIMIT ? OFFSET ?`
	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.IncidentListItem
	for rows.Next() {
		var item models.IncidentListItem
		var incType string
		var startedAt string
		var resolvedAt, ackedAt, ackedBy sql.NullString
		if err := rows.Scan(
			&item.ID, &item.MonitorID, &incType, &item.Message, &startedAt, &resolvedAt,
			&ackedAt, &ackedBy, &item.MonitorName,
		); err != nil {
			return nil, err
		}
		item.Type = models.IncidentType(incType)
		item.Message = models.StripHTTPErrorSourceSuffix(item.Message)
		applyIncidentTimes(&item.Incident, startedAt, resolvedAt, ackedAt, ackedBy)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CountIncidents(q IncidentQuery) (int, error) {
	sqlQ := `
		SELECT COUNT(*)
		FROM incidents i
		LEFT JOIN monitors m ON m.id = i.monitor_id
		LEFT JOIN performance_targets pt ON pt.id = i.monitor_id
		LEFT JOIN hosts h ON h.id = i.monitor_id`
	conds, args := incidentConds(q)
	if len(conds) > 0 {
		sqlQ += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	var n int
	err := s.db.QueryRow(sqlQ, args...).Scan(&n)
	return n, err
}

func incidentConds(q IncidentQuery) ([]string, []any) {
	var conds []string
	var args []any
	switch strings.ToLower(strings.TrimSpace(q.Status)) {
	case "open":
		conds = append(conds, `i.resolved_at IS NULL`)
	case "resolved":
		conds = append(conds, `i.resolved_at IS NOT NULL`)
	default:
		if q.OpenOnly {
			conds = append(conds, `i.resolved_at IS NULL`)
		}
	}
	if typ := strings.TrimSpace(q.Type); typ != "" {
		conds = append(conds, `i.type = ?`)
		args = append(args, typ)
	} else {
		// Outages are one row (down/ssl/…); recovery is email-only, not a separate incident.
		conds = append(conds, `i.type != ?`)
		args = append(args, string(models.IncidentRecovery))
	}
	if q.MonitorID != "" {
		conds = append(conds, `i.monitor_id = ?`)
		args = append(args, q.MonitorID)
	}
	if q.TenantID != "" {
		conds = append(conds, `(m.tenant_id = ? OR pt.tenant_id = ? OR h.tenant_id = ?)`)
		args = append(args, q.TenantID, q.TenantID, q.TenantID)
	}
	if q.From != nil {
		conds = append(conds, `i.started_at >= ?`)
		args = append(args, formatTime(*q.From))
	}
	if q.To != nil {
		conds = append(conds, `i.started_at < ?`)
		args = append(args, formatTime(*q.To))
	}
	return conds, args
}

func (s *Store) GetLastSlowAlertAt(monitorID string) (*time.Time, error) {
	var startedAt string
	err := s.db.QueryRow(`
		SELECT started_at FROM incidents
		WHERE monitor_id = ? AND type = ?
		ORDER BY started_at DESC LIMIT 1`,
		monitorID, string(models.IncidentSlow),
	).Scan(&startedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t, err := parseTime(startedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// PruneOldIncidentCaptures drops captured error-page HTML after the retention
// window. Incident rows stay for SLA; only resolved captures are removed.
func (s *Store) PruneOldIncidentCaptures(before time.Time) (int64, error) {
	res, err := s.db.Exec(`
		UPDATE incidents SET details = NULL
		WHERE details IS NOT NULL AND TRIM(details) != ''
			AND resolved_at IS NOT NULL AND resolved_at < ?`,
		formatTime(before),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
