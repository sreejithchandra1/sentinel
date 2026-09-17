package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/safehost"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func (s *Server) handleListPerformanceTargets(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	customerFilter := strings.TrimSpace(r.URL.Query().Get("customer"))
	var targets []models.PerformanceTargetListItem
	var err error
	if isPlatformAdmin(user) {
		if customerFilter != "" {
			targets, err = s.store.ListPerformanceTargetsByTenant(customerFilter)
		} else {
			targets, err = s.store.ListPerformanceTargets()
		}
	} else if user.TenantID != "" {
		targets, err = s.store.ListPerformanceTargetsByTenant(user.TenantID)
	} else {
		targets = []models.PerformanceTargetListItem{}
	}
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if targets == nil {
		targets = []models.PerformanceTargetListItem{}
	}
	for i := range targets {
		redactPerformanceTarget(&targets[i].PerformanceTarget, user)
	}
	jsonOK(w, targets)
}

func (s *Server) handleGetPerformanceTarget(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	t, err := s.store.GetPerformanceTarget(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if t == nil || !canAccessTenant(user, t.TenantID) || (!isPlatformAdmin(user) && t.TenantID == "") {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	redactPerformanceTarget(t, user)
	jsonOK(w, t)
}

func (s *Server) handleCreatePerformanceTarget(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var t models.PerformanceTarget
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if t.Name == "" || t.URL == "" {
		jsonError(w, http.StatusBadRequest, "name and url required")
		return
	}
	if err := safehost.ValidateHTTPURL(t.URL); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if t.IntervalSeconds < 30 {
		t.IntervalSeconds = 30
	}
	if isCustomerAdmin(user) {
		t.TenantID = user.TenantID
	} else if isPlatformAdmin(user) {
		t.TenantID = strings.TrimSpace(t.TenantID)
	} else {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	s.store.InheritPerformanceHTTPAuth(&t)
	if err := s.store.CreatePerformanceTarget(&t); err != nil {
		jsonInternal(w, err)
		return
	}
	redactPerformanceTarget(&t, user)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, t)
}

func (s *Server) handleUpdatePerformanceTarget(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	existing, err := s.store.GetPerformanceTarget(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if isCustomerAdmin(user) {
		if existing.TenantID != user.TenantID {
			jsonError(w, http.StatusNotFound, "not found")
			return
		}
	} else if !isPlatformAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}

	var input models.PerformanceTarget
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	existing.Name = input.Name
	existing.URL = input.URL
	if input.Method != "" {
		existing.Method = input.Method
	}
	if input.IntervalSeconds < 30 {
		existing.IntervalSeconds = 30
	} else {
		existing.IntervalSeconds = input.IntervalSeconds
	}
	if input.TimeoutMs > 0 {
		existing.TimeoutMs = input.TimeoutMs
	}
	if input.SlowThresholdMs > 0 {
		existing.SlowThresholdMs = input.SlowThresholdMs
	}
	existing.FollowRedirects = input.FollowRedirects
	existing.Enabled = input.Enabled
	existing.AlertEmails = input.AlertEmails
	applyPerformanceHTTPAuthUpdate(existing, &input)
	if input.AlertAfterSlow > 0 {
		existing.AlertAfterSlow = input.AlertAfterSlow
	}
	if isCustomerAdmin(user) {
		existing.TenantID = user.TenantID
	} else {
		existing.TenantID = strings.TrimSpace(input.TenantID)
	}
	if err := safehost.ValidateHTTPURL(existing.URL); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.FillPerformanceHTTPAuth(existing)

	if err := s.store.UpdatePerformanceTarget(existing); err != nil {
		jsonInternal(w, err)
		return
	}
	redactPerformanceTarget(existing, user)
	jsonOK(w, existing)
}

func (s *Server) handleDeletePerformanceTarget(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	existing, err := s.store.GetPerformanceTarget(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if isCustomerAdmin(user) {
		if existing.TenantID != user.TenantID {
			jsonError(w, http.StatusNotFound, "not found")
			return
		}
	} else if !isPlatformAdmin(user) {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := s.store.DeletePerformanceTarget(id); err != nil {
		jsonInternal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListPerformanceResults(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	t, err := s.store.GetPerformanceTarget(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if t == nil || !canAccessTenant(user, t.TenantID) || (!isPlatformAdmin(user) && t.TenantID == "") {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	limit := queryInt(r, "limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	from, to, err := parseIncidentDayRange(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Default to SLA breaches only; pass breaches=0 to list all probes.
	breachesOnly := true
	if raw := strings.TrimSpace(r.URL.Query().Get("breaches")); raw != "" {
		breachesOnly = raw == "1" || strings.EqualFold(raw, "true")
	}
	results, total, err := s.store.QueryPerformanceResults(store.PerformanceResultQuery{
		TargetID:     id,
		ThresholdMs:  t.SlowThresholdMs,
		From:         from,
		To:           to,
		Limit:        limit,
		Offset:       offset,
		BreachesOnly: breachesOnly,
	})
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if results == nil {
		results = []models.PerformanceResult{}
	}
	jsonOK(w, map[string]any{
		"items":  results,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetPerformanceStats(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	t, err := s.store.GetPerformanceTarget(id)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if t == nil || !canAccessTenant(user, t.TenantID) || (!isPlatformAdmin(user) && t.TenantID == "") {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	win := parseStatsRange(r.URL.Query().Get("period"), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	stats, err := s.store.GetPerformanceTargetStats(id, win.From, win.To)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, stats)
}
