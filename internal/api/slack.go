package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

func (s *Server) slackTenantFor(r *http.Request) (tenantID string, ok bool) {
	actor := currentUser(r)
	if isPlatformAdmin(actor) {
		return "", true
	}
	if isCustomerAdmin(actor) {
		return strings.TrimSpace(actor.TenantID), true
	}
	return "", false
}

func (s *Server) handleGetSlack(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.slackTenantFor(r)
	if !ok {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	cfg, err := s.store.GetSlackConfig(tenantID)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	jsonOK(w, cfg)
}

func (s *Server) handlePutSlack(w http.ResponseWriter, r *http.Request) {
	actor := currentUser(r)
	tenantID, ok := s.slackTenantFor(r)
	if !ok {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	var cfg models.SlackConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
	if cfg.WebhookURL != "" {
		if err := safehost.ValidateHTTPURL(cfg.WebhookURL); err != nil {
			jsonError(w, http.StatusBadRequest, "webhook URL is not allowed")
			return
		}
	}
	if cfg.Events == nil {
		cfg.Events = []string{"all"}
	}
	if err := s.store.SaveSlackConfig(tenantID, cfg); err != nil {
		jsonInternal(w, err)
		return
	}
	_ = s.store.InsertAudit(actor.Username, "update", "slack", tenantID)
	jsonOK(w, cfg)
}

func (s *Server) handleTestSlack(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.slackTenantFor(r)
	if !ok {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := s.alerter.SendTestSlack(tenantID); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// handleNotificationsSummary returns enable/configured flags for the hub.
func (s *Server) handleNotificationsSummary(w http.ResponseWriter, r *http.Request) {
	actor := currentUser(r)
	tenantID, ok := s.slackTenantFor(r)
	if !ok {
		jsonError(w, http.StatusForbidden, "forbidden")
		return
	}

	slack, err := s.store.GetSlackConfig(tenantID)
	if err != nil {
		jsonInternal(w, err)
		return
	}

	out := map[string]any{
		"slack": map[string]any{
			"enabled":     slack.Enabled,
			"configured":  strings.TrimSpace(slack.WebhookURL) != "",
			"webhook_url": slack.WebhookURL,
			"events":      slack.Events,
		},
	}

	smtp, err := s.store.GetSMTPConfig(s.defaultSMTP)
	if err != nil {
		jsonInternal(w, err)
		return
	}
	out["email"] = map[string]any{
		"enabled":    smtp.Enabled,
		"configured": strings.TrimSpace(smtp.Host) != "",
	}

	if isPlatformAdmin(actor) {
		hooks, _ := s.store.GetWebhooks()
		anyHookEnabled := false
		anyHookConfigured := false
		for _, h := range hooks {
			if strings.TrimSpace(h.URL) != "" {
				anyHookConfigured = true
				if h.Enabled {
					anyHookEnabled = true
				}
			}
		}
		out["webhooks"] = map[string]any{
			"enabled":    anyHookEnabled,
			"configured": anyHookConfigured,
			"count":      len(hooks),
		}
	}

	jsonOK(w, out)
}
