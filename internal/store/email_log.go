package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type EmailLogQuery struct {
	Status string
	Kind   string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

func (s *Store) InsertEmailLog(e *models.EmailLog) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Kind == "" {
		e.Kind = models.EmailKindAlert
	}
	_, err := s.db.Exec(`
		INSERT INTO email_log (id, status, kind, to_addr, subject, error, monitor_id, monitor_name, tenant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Status, e.Kind, e.ToAddr, e.Subject, e.Error,
		nullString(e.MonitorID), e.MonitorName, nullString(e.TenantID), formatTime(e.CreatedAt),
	)
	return err
}

func (s *Store) QueryEmailLog(q EmailLogQuery) ([]models.EmailLog, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	sqlQ := `SELECT id, status, kind, to_addr, subject, error, monitor_id, monitor_name, tenant_id, created_at FROM email_log`
	conds, args := emailLogConds(q)
	if len(conds) > 0 {
		sqlQ += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	sqlQ += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.EmailLog
	for rows.Next() {
		e, err := scanEmailLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CountEmailLog(q EmailLogQuery) (int, error) {
	sqlQ := `SELECT COUNT(*) FROM email_log`
	conds, args := emailLogConds(q)
	if len(conds) > 0 {
		sqlQ += ` WHERE ` + strings.Join(conds, ` AND `)
	}
	var n int
	err := s.db.QueryRow(sqlQ, args...).Scan(&n)
	return n, err
}

func (s *Store) PruneOldEmailLog(before time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM email_log WHERE created_at < ?`, formatTime(before))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func emailLogConds(q EmailLogQuery) ([]string, []any) {
	var conds []string
	var args []any
	if s := strings.TrimSpace(q.Status); s != "" {
		conds = append(conds, `status = ?`)
		args = append(args, s)
	}
	if k := strings.TrimSpace(q.Kind); k != "" {
		conds = append(conds, `kind = ?`)
		args = append(args, k)
	}
	if q.From != nil {
		conds = append(conds, `created_at >= ?`)
		args = append(args, formatTime(*q.From))
	}
	if q.To != nil {
		conds = append(conds, `created_at < ?`)
		args = append(args, formatTime(*q.To))
	}
	return conds, args
}

func scanEmailLog(row interface{ Scan(dest ...any) error }) (models.EmailLog, error) {
	var e models.EmailLog
	var monitorID, tenantID sql.NullString
	var created string
	err := row.Scan(
		&e.ID, &e.Status, &e.Kind, &e.ToAddr, &e.Subject, &e.Error,
		&monitorID, &e.MonitorName, &tenantID, &created,
	)
	if err != nil {
		return e, err
	}
	e.MonitorID = nullableString(monitorID)
	e.TenantID = nullableString(tenantID)
	e.CreatedAt, _ = parseTime(created)
	return e, nil
}
