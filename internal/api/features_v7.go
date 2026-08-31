package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/config"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	openOnly := r.URL.Query().Get("open") == "1"
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
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if openOnly && status == "" {
		status = "open"
	}
	q := store.IncidentQuery{
		Limit:     limit,
		Offset:    offset,
		OpenOnly:  openOnly,
		Status:    status,
		Type:      strings.TrimSpace(r.URL.Query().Get("type")),
		MonitorID: strings.TrimSpace(r.URL.Query().Get("monitor_id")),
		From:      from,
		To:        to,
	}
	var items []models.IncidentListItem
	if isPlatformAdmin(user) {
		items, err = s.store.QueryIncidents(q)
	} else if user.TenantID != "" {
		q.TenantID = user.TenantID
		items, err = s.store.QueryIncidents(q)
	} else {
		items = []models.IncidentListItem{}
	}
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if items == nil {
		items = []models.IncidentListItem{}
	}
	total, err := s.store.CountIncidents(q)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func incidentVisible(user *models.User, tenantID string) bool {
	if user == nil {
		return false
	}
	if isPlatformAdmin(user) {
		return true
	}
	if tenantID == "" {
		return false
	}
	return canAccessTenant(user, tenantID)
}

func (s *Server) loadVisibleIncident(w http.ResponseWriter, r *http.Request) *models.IncidentListItem {
	user := currentUser(r)
	id := r.PathValue("id")
	item, tenantID, err := s.store.GetIncident(id)
	if err != nil {
		jsonInternal(w, err)
		return nil
	}
	if item == nil || !incidentVisible(user, tenantID) {
		jsonError(w, http.StatusNotFound, "not found")
		return nil
	}
	return item
}

func ackActorName(u *models.User) string {
	if u == nil {
		return ""
	}
	if name := strings.TrimSpace(u.Name); name != "" {
		return name
	}
	return u.Username
}

func (s *Server) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	item := s.loadVisibleIncident(w, r)
	if item == nil {
		return
	}
	jsonOK(w, item)
}

func (s *Server) handleAcknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	item := s.loadVisibleIncident(w, r)
	if item == nil {
		return
	}
	user := currentUser(r)
	acked, err := s.store.AcknowledgeIncident(item.ID, ackActorName(user), time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrIncidentResolved) {
			jsonError(w, http.StatusConflict, "incident already resolved")
			return
		}
		jsonInternal(w, err)
		return
	}
	if acked == nil {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	_ = s.store.InsertAudit(user.Username, "update", "incident", item.ID)
	jsonOK(w, acked)
}

func (s *Server) handleGetWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := s.store.GetWebhooks()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if hooks == nil {
		hooks = []models.WebhookConfig{}
	}
	jsonOK(w, hooks)
}

func (s *Server) handlePutWebhooks(w http.ResponseWriter, r *http.Request) {
	var hooks []models.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&hooks); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := s.store.SaveWebhooks(hooks); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "update", "webhooks", "")
	jsonOK(w, hooks)
}

func (s *Server) handleListMaintenance(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMaintenanceWindows()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if items == nil {
		items = []models.MaintenanceWindow{}
	}
	jsonOK(w, items)
}

func (s *Server) handleCreateMaintenance(w http.ResponseWriter, r *http.Request) {
	var wnd models.MaintenanceWindow
	if err := json.NewDecoder(r.Body).Decode(&wnd); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if wnd.Name == "" || wnd.EndsAt.Before(wnd.StartsAt) {
		jsonError(w, http.StatusBadRequest, "invalid maintenance window")
		return
	}
	if err := s.store.CreateMaintenanceWindow(&wnd); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "create", "maintenance", wnd.Name)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, wnd)
}

func (s *Server) handleDeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteMaintenanceWindow(id); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "delete", "maintenance", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetServerSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetServerSettings(s.serverFallback())
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, cfg)
}

func (s *Server) handlePutServerSettings(w http.ResponseWriter, r *http.Request) {
	var cfg models.ServerSettings
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if cfg.RetentionDays < 30 {
		jsonError(w, http.StatusBadRequest, "retention_days must be at least 30")
		return
	}
	if err := s.store.SaveServerSettings(cfg); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "update", "server", "")
	jsonOK(w, cfg)
}

func (s *Server) handleGetStatusPageConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetStatusPageConfig()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, cfg)
}

func (s *Server) handlePutStatusPageConfig(w http.ResponseWriter, r *http.Request) {
	var cfg models.StatusPageConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if cfg.Title == "" {
		cfg.Title = "System Status"
	}
	if cfg.MonitorIDs == nil {
		cfg.MonitorIDs = []string{}
	}
	if err := s.store.SaveStatusPageConfig(cfg); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "update", "status_page", "")
	jsonOK(w, cfg)
}

func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetStatusPageConfig()
	if err != nil || !cfg.Enabled {
		jsonError(w, http.StatusNotFound, "status page not available")
		return
	}
	title := cfg.Title
	if title == "" {
		title = "System Status"
	}

	var monitors []models.PublicMonitorStatus
	for _, id := range cfg.MonitorIDs {
		m, err := s.store.GetMonitor(id)
		if err != nil || m == nil {
			continue
		}
		item := models.PublicMonitorStatus{
			ID:     m.ID,
			Name:   m.Name,
			Type:   string(m.Type),
			Status: string(m.LastStatus),
		}
		if m.LastCheckedAt != nil {
			item.LastCheck = m.LastCheckedAt.UTC().Format(time.RFC3339)
		}
		monitors = append(monitors, item)
	}

	jsonOK(w, models.PublicStatusResponse{Title: title, Monitors: monitors})
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
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
	q := store.AuditQuery{
		Actor:    strings.TrimSpace(r.URL.Query().Get("actor")),
		Action:   strings.TrimSpace(r.URL.Query().Get("action")),
		Resource: strings.TrimSpace(r.URL.Query().Get("resource")),
		From:     from,
		To:       to,
		Limit:    limit,
		Offset:   offset,
	}
	items, err := s.store.QueryAuditLog(q)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if items == nil {
		items = []models.AuditEntry{}
	}
	total, err := s.store.CountAuditLog(q)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleListAuditMeta(w http.ResponseWriter, r *http.Request) {
	actors, err := s.store.ListAuditActors()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	resources, err := s.store.ListAuditResources()
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if actors == nil {
		actors = []string{}
	}
	if resources == nil {
		resources = []string{}
	}
	jsonOK(w, map[string]any{
		"actors":    actors,
		"resources": resources,
		"actions": []string{
			"create", "update", "delete",
			"lockout", "rate_limit", "unlock", "password_reset",
		},
	})
}

func (s *Server) handleListAPITokens(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	tokens, err := s.store.ListAPITokens(user.ID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if tokens == nil {
		tokens = []models.APIToken{}
	}
	jsonOK(w, tokens)
}

type createTokenRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name required")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token error")
		return
	}
	created, err := s.store.CreateAPIToken(currentUser(r).ID, req.Name, token)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "create", "api_token", req.Name)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, created)
}

func (s *Server) handleDeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAPIToken(id, currentUser(r).ID); err != nil {
		jsonError(w, http.StatusNotFound, "token not found")
		return
	}
	_ = s.store.InsertAudit(currentUser(r).Username, "delete", "api_token", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHeartbeatPing(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	m, err := s.store.GetMonitorByHeartbeatToken(token)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	if m == nil || !m.Enabled {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}

	now := time.Now().UTC()
	result := &models.CheckResult{
		MonitorID:      m.ID,
		Status:         models.StatusUp,
		ResponseTimeMs: 0,
		CheckedAt:      now,
	}
	if err := s.store.InsertCheckResult(result); err != nil {
		jsonInternal(w, err)
		return
	}
	prevStatus := m.LastStatus
	m.LastCheckedAt = &now
	m.LastStatus = models.StatusUp
	m.ConsecutiveFailures = 0
	if err := s.store.UpdateMonitorState(m.ID, models.StatusUp, 0, now); err != nil {
		jsonInternal(w, err)
		return
	}
	if prevStatus == models.StatusDown {
		open, _ := s.store.GetOpenIncident(m.ID, models.IncidentDown)
		if open != nil {
			_ = s.store.ResolveIncident(open.ID, now)
			_ = s.alerter.NotifyMonitor(m, "RECOVERY", "Heartbeat received", 0)
		}
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (s *Server) serverFallback() config.ServerConfig {
	return config.ServerConfig{
		DashboardURL:  s.dashboardURL,
		RetentionDays: 30,
		Workers:       10,
	}
}

func (s *Server) dashboardBaseURL() string {
	cfg, err := s.store.GetServerSettings(s.serverFallback())
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(s.dashboardURL), "/")
	}
	if u := strings.TrimRight(strings.TrimSpace(cfg.DashboardURL), "/"); u != "" {
		return u
	}
	return strings.TrimRight(strings.TrimSpace(s.dashboardURL), "/")
}

func parseTagsInput(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
