package api

import (
	"net/http"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func (s *Server) handleGetFleetPerformance(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	win := parseStatsRange(r.URL.Query().Get("period"), r.URL.Query().Get("from"), r.URL.Query().Get("to"))

	var fleet *models.FleetPerformance
	var err error
	if isPlatformAdmin(user) {
		customerFilter := strings.TrimSpace(r.URL.Query().Get("customer"))
		if customerFilter != "" {
			fleet, err = s.store.GetFleetPerformanceByTenant(win.From, win.To, customerFilter)
		} else {
			fleet, err = s.store.GetFleetPerformance(win.From, win.To)
		}
	} else if user.TenantID != "" {
		fleet, err = s.store.GetFleetPerformanceByTenant(win.From, win.To, user.TenantID)
	} else {
		fleet = &models.FleetPerformance{Monitors: []models.MonitorPerformance{}, Timeline: []models.FleetTimelinePoint{}}
	}
	if err != nil {
		jsonInternal(w, err)
		return
	}
	fleet.Period = win.Period
	if fleet.Monitors == nil {
		fleet.Monitors = []models.MonitorPerformance{} // json: services
	}
	if fleet.Timeline == nil {
		fleet.Timeline = []models.FleetTimelinePoint{}
	}
	jsonOK(w, fleet)
}
