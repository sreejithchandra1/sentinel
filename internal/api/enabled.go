package api

import (
	"encoding/json"
	"net/http"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type enabledPayload struct {
	Enabled *bool `json:"enabled"`
}

func (s *Server) handleSetMonitorEnabled(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	existing, ok := s.loadWritableMonitor(w, r)
	if !ok {
		return
	}
	enabled, ok := decodeEnabled(w, r)
	if !ok {
		return
	}
	if err := s.store.SetMonitorEnabled(existing.ID, enabled); err != nil {
		jsonInternal(w, err)
		return
	}
	updated, err := s.store.GetMonitor(existing.ID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if updated == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	action := "pause"
	if enabled {
		action = "resume"
	}
	_ = s.store.InsertAudit(user.Username, action, "monitor", updated.Name)
	redactMonitor(updated, user)
	jsonOK(w, updated)
}

func (s *Server) handleSetPerformanceTargetEnabled(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	existing, ok := s.loadWritablePerformanceTarget(w, r)
	if !ok {
		return
	}
	enabled, ok := decodeEnabled(w, r)
	if !ok {
		return
	}
	if err := s.store.SetPerformanceTargetEnabled(existing.ID, enabled); err != nil {
		jsonInternal(w, err)
		return
	}
	updated, err := s.store.GetPerformanceTarget(existing.ID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if updated == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	action := "pause"
	if enabled {
		action = "resume"
	}
	_ = s.store.InsertAudit(user.Username, action, "performance", updated.Name)
	redactPerformanceTarget(updated, user)
	jsonOK(w, updated)
}

func decodeEnabled(w http.ResponseWriter, r *http.Request) (bool, bool) {
	var body enabledPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return false, false
	}
	if body.Enabled == nil {
		jsonError(w, http.StatusBadRequest, "enabled required")
		return false, false
	}
	return *body.Enabled, true
}

func (s *Server) loadWritableMonitor(w http.ResponseWriter, r *http.Request) (*models.Monitor, bool) {
	user := currentUser(r)
	existing, err := s.store.GetMonitor(r.PathValue("id"))
	if err != nil {
		jsonInternal(w, err)
		return nil, false
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	if !canWriteMonitor(user, existing.TenantID) {
		if isCustomerAdmin(user) {
			jsonError(w, http.StatusNotFound, "not found")
			return nil, false
		}
		jsonError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return existing, true
}

func (s *Server) loadWritablePerformanceTarget(w http.ResponseWriter, r *http.Request) (*models.PerformanceTarget, bool) {
	user := currentUser(r)
	existing, err := s.store.GetPerformanceTarget(r.PathValue("id"))
	if err != nil {
		jsonInternal(w, err)
		return nil, false
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	if !canWriteMonitor(user, existing.TenantID) {
		if isCustomerAdmin(user) {
			jsonError(w, http.StatusNotFound, "not found")
			return nil, false
		}
		jsonError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return existing, true
}

func canWriteMonitor(user *models.User, tenantID string) bool {
	if isCustomerAdmin(user) {
		return tenantID == user.TenantID
	}
	return isPlatformAdmin(user)
}
