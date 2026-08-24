package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func parseSLAMonth(raw string, now time.Time) (from, to time.Time, err error) {
	now = now.UTC()
	var year int
	var month time.Month
	raw = strings.TrimSpace(raw)
	if raw == "" {
		year, month = now.Year(), now.Month()
	} else {
		t, perr := time.Parse("2006-01", raw)
		if perr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid month")
		}
		year, month = t.Year(), t.Month()
	}
	from = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	next := from.AddDate(0, 1, 0)
	to = next
	if now.Before(from) {
		return from, from, nil
	}
	if now.Before(next) {
		to = now
	}
	return from, to, nil
}

func (s *Server) handleGetSLAReport(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	from, to, err := parseSLAMonth(r.URL.Query().Get("month"), time.Now())
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	customerFilter := strings.TrimSpace(r.URL.Query().Get("customer"))

	var tenantID string
	if isPlatformAdmin(user) {
		tenantID = customerFilter
	} else if user.TenantID != "" {
		tenantID = user.TenantID
	} else {
		jsonOK(w, models.SLAReport{Monitors: []models.SLAMonitorRow{}, AvailabilityPct: 100, PeriodStart: from, PeriodEnd: to})
		return
	}

	rep, err := s.store.GetSLAReport(tenantID, from, to)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, rep)
}
