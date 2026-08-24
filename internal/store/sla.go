package store

import (
	"database/sql"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func overlapSeconds(aStart, aEnd, bStart, bEnd time.Time) int64 {
	start := aStart
	if bStart.After(start) {
		start = bStart
	}
	end := aEnd
	if bEnd.Before(end) {
		end = bEnd
	}
	if !end.After(start) {
		return 0
	}
	return int64(end.Sub(start) / time.Second)
}

func maintenanceOverlapSeconds(monitorID string, start, end time.Time, windows []models.MaintenanceWindow) int64 {
	var n int64
	for _, w := range windows {
		if w.MonitorID != "" && w.MonitorID != monitorID {
			continue
		}
		n += overlapSeconds(start, end, w.StartsAt, w.EndsAt)
	}
	return n
}

func (s *Store) ListDownIncidentsOverlapping(tenantID string, from, to time.Time) ([]models.IncidentListItem, error) {
	q := `
		SELECT i.id, i.monitor_id, i.type, i.message, i.started_at, i.resolved_at,
			COALESCE(m.name, '') AS monitor_name
		FROM incidents i
		INNER JOIN monitors m ON m.id = i.monitor_id
		WHERE i.type = ?
			AND i.started_at < ?
			AND (i.resolved_at IS NULL OR i.resolved_at > ?)`
	args := []any{string(models.IncidentDown), formatTime(to), formatTime(from)}
	if tenantID != "" {
		q += ` AND m.tenant_id = ?`
		args = append(args, tenantID)
	}
	q += ` ORDER BY i.started_at ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.IncidentListItem
	for rows.Next() {
		var item models.IncidentListItem
		var incType string
		var startedAt string
		var resolvedAt sql.NullString
		if err := rows.Scan(&item.ID, &item.MonitorID, &incType, &item.Message, &startedAt, &resolvedAt, &item.MonitorName); err != nil {
			return nil, err
		}
		item.Type = models.IncidentType(incType)
		item.StartedAt, _ = parseTime(startedAt)
		if resolvedAt.Valid && resolvedAt.String != "" {
			if t, err := parseTime(resolvedAt.String); err == nil {
				item.ResolvedAt = &t
			}
		}
		out = append(out, item)
	}
	if out == nil {
		out = []models.IncidentListItem{}
	}
	return out, rows.Err()
}

func (s *Store) GetSLAReport(tenantID string, from, to time.Time) (*models.SLAReport, error) {
	var monitors []models.MonitorListItem
	var err error
	if tenantID == "" {
		monitors, err = s.ListMonitors()
	} else {
		monitors, err = s.ListMonitorsByTenant(tenantID)
	}
	if err != nil {
		return nil, err
	}
	incidents, err := s.ListDownIncidentsOverlapping(tenantID, from, to)
	if err != nil {
		return nil, err
	}
	windows, err := s.ListMaintenanceWindows()
	if err != nil {
		return nil, err
	}
	rep := BuildSLAReport(monitors, incidents, windows, from, to)
	return &rep, nil
}

func BuildSLAReport(
	monitors []models.MonitorListItem,
	incidents []models.IncidentListItem,
	windows []models.MaintenanceWindow,
	from, to time.Time,
) models.SLAReport {
	if to.Before(from) {
		to = from
	}
	windowSec := int64(to.Sub(from) / time.Second)
	if windowSec < 0 {
		windowSec = 0
	}

	type acc struct {
		name      string
		tenantID  string
		downtime  int64
		incidents int
		mttrSum   float64
		mttrN     int
	}
	byID := make(map[string]*acc, len(monitors))
	order := make([]string, 0, len(monitors))
	for _, m := range monitors {
		byID[m.ID] = &acc{name: m.Name, tenantID: m.TenantID}
		order = append(order, m.ID)
	}

	for _, inc := range incidents {
		row, ok := byID[inc.MonitorID]
		if !ok {
			continue
		}
		row.incidents++
		end := to
		if inc.ResolvedAt != nil && inc.ResolvedAt.Before(end) {
			end = *inc.ResolvedAt
		}
		start := inc.StartedAt
		if start.Before(from) {
			start = from
		}
		raw := overlapSeconds(start, end, from, to)
		maint := maintenanceOverlapSeconds(inc.MonitorID, start, end, windows)
		d := raw - maint
		if d < 0 {
			d = 0
		}
		row.downtime += d
		if inc.ResolvedAt != nil && !inc.ResolvedAt.Before(from) && inc.ResolvedAt.Before(to) {
			row.mttrSum += inc.ResolvedAt.Sub(inc.StartedAt).Seconds()
			row.mttrN++
		}
	}

	out := models.SLAReport{
		PeriodStart:     from,
		PeriodEnd:       to,
		MonitorCount:    len(monitors),
		WindowSeconds:   windowSec,
		Monitors:        make([]models.SLAMonitorRow, 0, len(order)),
		AvailabilityPct: 100,
	}
	var availSum float64
	var mttrSum float64
	var mttrN int
	for _, id := range order {
		row := byID[id]
		if row.downtime > windowSec && windowSec > 0 {
			row.downtime = windowSec
		}
		pct := 100.0
		if windowSec > 0 {
			pct = (1 - float64(row.downtime)/float64(windowSec)) * 100
			if pct < 0 {
				pct = 0
			}
		}
		availSum += pct
		out.IncidentCount += row.incidents
		out.DowntimeSeconds += row.downtime
		item := models.SLAMonitorRow{
			MonitorID:       id,
			Name:            row.name,
			TenantID:        row.tenantID,
			IncidentCount:   row.incidents,
			DowntimeSeconds: row.downtime,
			AvailabilityPct: pct,
		}
		if row.mttrN > 0 {
			v := row.mttrSum / float64(row.mttrN)
			item.MTTRSeconds = &v
			mttrSum += row.mttrSum
			mttrN += row.mttrN
		}
		out.Monitors = append(out.Monitors, item)
	}
	if len(order) > 0 {
		out.AvailabilityPct = availSum / float64(len(order))
	}
	if mttrN > 0 {
		v := mttrSum / float64(mttrN)
		out.MTTRSeconds = &v
	}
	if out.Monitors == nil {
		out.Monitors = []models.SLAMonitorRow{}
	}
	return out
}
