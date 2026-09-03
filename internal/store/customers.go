package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

var ErrMonitorQuotaExceeded = errors.New("Monitor Quota Exceeded")

func (s *Store) ListCustomers() ([]models.Customer, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.monitor_quota, c.alert_emails, c.created_at,
			(SELECT COUNT(*) FROM monitors m WHERE m.tenant_id = c.id) AS monitor_count
		FROM customers c
		ORDER BY c.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Customer
	for rows.Next() {
		c, err := scanCustomer(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCustomer(id string) (*models.Customer, error) {
	row := s.db.QueryRow(`
		SELECT c.id, c.name, c.monitor_quota, c.alert_emails, c.created_at,
			(SELECT COUNT(*) FROM monitors m WHERE m.tenant_id = c.id) AS monitor_count
		FROM customers c WHERE c.id = ?`, id)
	c, err := scanCustomer(row, true)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateCustomer(name string, monitorQuota int) (*models.Customer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if monitorQuota < 1 {
		monitorQuota = 1
	}
	now := time.Now().UTC()
	id := newID()
	_, err := s.db.Exec(`
		INSERT INTO customers (id, name, monitor_quota, alert_emails, created_at)
		VALUES (?, ?, ?, '', ?)`,
		id, name, monitorQuota, formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return s.GetCustomer(id)
}

func (s *Store) UpdateCustomer(id, name string, monitorQuota int, alertEmails string) (*models.Customer, error) {
	existing, err := s.GetCustomer(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("customer not found")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if monitorQuota < 1 {
		return nil, fmt.Errorf("monitor_quota must be at least 1")
	}
	_, err = s.db.Exec(`UPDATE customers SET name = ?, monitor_quota = ?, alert_emails = ? WHERE id = ?`,
		name, monitorQuota, strings.TrimSpace(alertEmails), id)
	if err != nil {
		return nil, err
	}
	return s.GetCustomer(id)
}

func (s *Store) DeleteCustomer(id string) error {
	existing, err := s.GetCustomer(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("customer not found")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE monitors SET tenant_id = NULL WHERE tenant_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE performance_targets SET tenant_id = NULL WHERE tenant_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET tenant_id = NULL WHERE tenant_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM customers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("customer not found")
	}
	return tx.Commit()
}

func (s *Store) CountMonitorsByTenant(tenantID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM monitors WHERE tenant_id = ?`, tenantID).Scan(&n)
	return n, err
}

// AssertMonitorQuota returns ErrMonitorQuotaExceeded if the tenant is at capacity.
func (s *Store) AssertMonitorQuota(tenantID string) error {
	if tenantID == "" {
		return nil
	}
	c, err := s.GetCustomer(tenantID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("customer not found")
	}
	count, err := s.CountMonitorsByTenant(tenantID)
	if err != nil {
		return err
	}
	if count >= c.MonitorQuota {
		return fmt.Errorf("%w (%d/%d)", ErrMonitorQuotaExceeded, count, c.MonitorQuota)
	}
	return nil
}

func scanCustomer(row interface{ Scan(dest ...any) error }, withCount bool) (models.Customer, error) {
	var c models.Customer
	var createdAt string
	var err error
	if withCount {
		err = row.Scan(&c.ID, &c.Name, &c.MonitorQuota, &c.AlertEmails, &createdAt, &c.MonitorCount)
	} else {
		err = row.Scan(&c.ID, &c.Name, &c.MonitorQuota, &c.AlertEmails, &createdAt)
	}
	if err != nil {
		return c, err
	}
	c.CreatedAt, err = parseTime(createdAt)
	return c, err
}
