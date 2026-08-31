package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func (a *Alerter) inMaintenance(monitorID string) bool {
	ok, err := a.store.IsInMaintenance(monitorID, time.Now().UTC())
	return err == nil && ok
}

func (a *Alerter) fireWebhooks(event string, payload map[string]any) {
	hooks, err := a.store.GetWebhooks()
	if err != nil {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, hook := range hooks {
		if !hook.Enabled || hook.URL == "" {
			continue
		}
		if !webhookMatchesEvent(hook.Events, event) {
			continue
		}
		go func(url string) {
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
		}(hook.URL)
	}
}

func webhookMatchesEvent(events []string, event string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if strings.EqualFold(e, event) || strings.EqualFold(e, "all") {
			return true
		}
	}
	return false
}

func (a *Alerter) NotifyMonitor(m *models.Monitor, alertType, message string, responseMs int) error {
	return a.NotifyMonitorMeta(m, AlertMeta{
		Event:      alertType,
		Message:    message,
		ResponseMs: responseMs,
		EventAt:    time.Now().UTC(),
	})
}

func (a *Alerter) NotifyMonitorMeta(m *models.Monitor, meta AlertMeta) error {
	if a.inMaintenance(m.ID) {
		log.Printf("alerter: %s alert skipped for %s: in maintenance", meta.Event, m.Name)
		return nil
	}
	meta.Name = m.Name
	meta.URL = m.URL
	if meta.DashboardURL == "" {
		meta.DashboardURL = a.liveDashboardURL() + "/monitors/" + m.ID
	}
	if meta.EventAt.IsZero() {
		meta.EventAt = time.Now().UTC()
	}

	payload := map[string]any{
		"event":            meta.Event,
		"monitor_id":       m.ID,
		"monitor_name":     m.Name,
		"monitor_type":     m.Type,
		"url":              m.URL,
		"message":          meta.Message,
		"response_time_ms": meta.ResponseMs,
		"dashboard_url":    meta.DashboardURL,
		"timestamp":        meta.EventAt.UTC().Format(time.RFC3339),
		"incident_id":      meta.IncidentID,
		"incident_label":   meta.IncidentLabel(),
	}
	if dt := meta.DowntimeLabel(); dt != "" {
		payload["downtime"] = dt
	}

	var emailErr error
	if m.NotifyEmail {
		if err := a.sendAlertMeta(m, meta); err != nil {
			emailErr = err
			log.Printf("alerter: email %s for %s: %v", meta.Event, m.Name, err)
		}
	}
	if m.NotifySlack {
		a.fireSlack(m.TenantID, meta)
	}
	if m.NotifyWebhooks {
		a.fireWebhooks(meta.Event, payload)
	}
	return emailErr
}

func (a *Alerter) HandlePerformanceResult(t *models.PerformanceTarget, result *models.PerformanceResult, prevStatus models.MonitorStatus) error {
	if a.inMaintenance(t.ID) {
		return nil
	}

	prev := prevStatus
	newStatus := result.Status
	if newStatus == models.StatusDown {
		newStatus = models.StatusDegraded
	}
	if prev == models.StatusDown {
		prev = models.StatusDegraded
	}

	wasSlow := prev == models.StatusDegraded
	isSlow := newStatus == models.StatusDegraded
	threshold := t.AlertAfterSlow
	if threshold < 1 {
		threshold = 1
	}
	consecutive := t.ConsecutiveSlow

	if isSlow {
		openSlow, _ := a.store.GetOpenIncident(t.ID, models.IncidentSlow)
		if openSlow != nil {
			return nil
		}
		if consecutive < threshold {
			return nil
		}
		// Fire once when crossing the consecutive threshold (or when already at/above after recovery gap).
		if wasSlow && consecutive != threshold {
			return nil
		}
		pct, total, slow, _ := a.store.GetPerformanceSlowStats(t.ID, time.Now().Add(-time.Hour))
		if total == 0 {
			pct = 100
			total = 1
			slow = 1
		}
		msg := fmt.Sprintf("%.1f%% of checks slow (%d of %d in the last hour); %d consecutive slow check(s)", pct, slow, total, consecutive)
		inc := &models.Incident{
			MonitorID: t.ID, Type: models.IncidentSlow, Message: msg, StartedAt: result.CheckedAt,
		}
		_ = a.store.CreateIncident(inc)
		return a.sendPerformanceAlert(t, AlertMeta{
			Event:      "SLOW",
			Message:    msg,
			ResponseMs: result.ResponseTimeMs,
			IncidentID: inc.ID,
			EventAt:    result.CheckedAt,
		})
	}

	if !isSlow && wasSlow {
		openSlow, _ := a.store.GetOpenIncident(t.ID, models.IncidentSlow)
		hadSlow, _ := a.store.HasOpenIncident(t.ID, models.IncidentSlow)
		hadDown, _ := a.store.HasOpenIncident(t.ID, models.IncidentDown)
		if hadSlow || hadDown {
			meta := AlertMeta{
				Event:      "NORMAL",
				Message:    "Back to normal",
				ResponseMs: result.ResponseTimeMs,
				EventAt:    result.CheckedAt,
			}
			if openSlow != nil {
				meta.IncidentID = openSlow.ID
				started := openSlow.StartedAt
				meta.StartedAt = &started
			}
			_ = a.store.ResolveOpenIncidents(t.ID, models.IncidentSlow, result.CheckedAt)
			_ = a.store.ResolveOpenIncidents(t.ID, models.IncidentDown, result.CheckedAt)
			return a.sendPerformanceAlert(t, meta)
		}
	}

	return nil
}

func (a *Alerter) sendPerformanceAlert(t *models.PerformanceTarget, meta AlertMeta) error {
	meta.Name = t.Name
	meta.URL = t.URL
	meta.DashboardURL = a.liveDashboardURL() + "/performance/" + t.ID
	if meta.EventAt.IsZero() {
		meta.EventAt = time.Now().UTC()
	}

	payload := map[string]any{
		"event":            meta.Event,
		"target_id":        t.ID,
		"target_name":      t.Name,
		"url":              t.URL,
		"message":          meta.Message,
		"response_time_ms": meta.ResponseMs,
		"dashboard_url":    meta.DashboardURL,
		"timestamp":        meta.EventAt.UTC().Format(time.RFC3339),
		"incident_id":      meta.IncidentID,
		"incident_label":   meta.IncidentLabel(),
	}
	if dt := meta.DowntimeLabel(); dt != "" {
		payload["downtime"] = dt
	}

	var emailErr error
	a.refreshSMTP()
	if a.cfg.Enabled && a.cfg.Host != "" {
		recipients := a.perfRecipients(t)
		if len(recipients) == 0 {
			emailErr = fmt.Errorf("no alert recipients — set alert emails on the target, SMTP Alert Recipients, or a profile email")
			log.Printf("alerter: email %s for %s: %v", meta.Event, t.Name, emailErr)
		} else {
			subject := meta.FallbackText()
			body := a.renderAlertEmail(meta)
			for _, to := range recipients {
				if err := a.sendSMTP(to, subject, body); err != nil {
					emailErr = err
					log.Printf("alerter: email %s for %s: %v", meta.Event, t.Name, err)
					break
				}
			}
		}
	}

	a.fireSlack(t.TenantID, meta)
	a.fireWebhooks(meta.Event, payload)
	return emailErr
}

func (a *Alerter) perfRecipients(t *models.PerformanceTarget) []string {
	if emails := parseEmails(t.AlertEmails); len(emails) > 0 {
		return emails
	}
	return a.defaultRecipients()
}
