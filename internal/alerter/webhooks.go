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
			client := outboundHTTPClient(10 * time.Second)
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
		a.recordEmailSkip(alertLogMeta(m), "", meta.FallbackText(), "in maintenance")
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
	if meta.ErrorPageURL != "" {
		payload["error_page_url"] = meta.ErrorPageURL
	}

	var emailErr error
	if m.NotifyEmail {
		if err := a.sendAlertMeta(m, meta); err != nil {
			emailErr = err
			log.Printf("alerter: email %s for %s: %v", meta.Event, m.Name, err)
		}
	} else {
		a.recordEmailSkip(alertLogMeta(m), "", meta.FallbackText(), "email channel off for this monitor")
	}
	if m.NotifySlack {
		a.fireSlackForAlert(m.TenantID, meta)
	}
	if m.NotifyWebhooks {
		a.fireWebhooks(meta.Event, payload)
	}
	return emailErr
}

func (a *Alerter) HandlePerformanceResult(t *models.PerformanceTarget, result *models.PerformanceResult, prevStatus models.MonitorStatus) error {
	if a.inMaintenance(t.ID) {
		a.recordEmailSkip(perfLogMeta(t), "", slowSubject(t.Name), "in maintenance")
		return nil
	}

	newStatus := result.Status
	threshold := performanceAlertAfterSlow(t)
	consecutive := t.ConsecutiveSlow
	isSlow := newStatus == models.StatusDegraded
	isUp := newStatus == models.StatusUp

	openSlow, err := a.store.GetOpenIncident(t.ID, models.IncidentSlow)
	if err != nil {
		return err
	}

	if isSlow {
		a.clearRecoveryStreak(t.ID)
		if openSlow != nil {
			a.recordEmailSkip(perfLogMeta(t), "", slowSubject(t.Name), "incident already open")
			return nil
		}
		if consecutive < threshold {
			a.recordEmailPending(perfLogMeta(t), slowSubject(t.Name),
				fmt.Sprintf("%d/%d consecutive slow checks", consecutive, threshold))
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
		if err := a.store.CreateIncident(inc); err != nil {
			return err
		}
		return a.sendPerformanceAlert(t, AlertMeta{
			Event:      "SLOW",
			Message:    msg,
			ResponseMs: result.ResponseTimeMs,
			IncidentID: inc.ID,
			EventAt:    result.CheckedAt,
		})
	}

	if openSlow == nil {
		a.clearRecoveryStreak(t.ID)
		return nil
	}
	if !isUp {
		a.clearRecoveryStreak(t.ID)
		return nil
	}

	streak := a.incRecoveryStreak(t.ID)
	if streak < threshold {
		a.recordEmailPending(perfLogMeta(t), AlertMeta{Event: "NORMAL", Name: t.Name}.FallbackText(),
			fmt.Sprintf("%d/%d successful checks", streak, threshold))
		return nil
	}
	a.clearRecoveryStreak(t.ID)
	started := openSlow.StartedAt
	_ = a.store.ResolveOpenIncidents(t.ID, models.IncidentSlow, result.CheckedAt)
	_ = a.store.ResolveOpenIncidents(t.ID, models.IncidentDown, result.CheckedAt)
	return a.sendPerformanceAlert(t, AlertMeta{
		Event:      "NORMAL",
		Message:    "Back to normal",
		ResponseMs: result.ResponseTimeMs,
		IncidentID: openSlow.ID,
		EventAt:    result.CheckedAt,
		StartedAt:  &started,
	})
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
	logMeta := perfLogMeta(t)
	subject := meta.FallbackText()
	if !a.cfg.Enabled {
		a.recordEmailSkip(logMeta, "", subject, "email alerts disabled")
	} else if a.cfg.Host == "" {
		a.recordEmailSkip(logMeta, "", subject, "SMTP not configured")
	} else {
		recipients := a.perfRecipients(t)
		if len(recipients) == 0 {
			emailErr = fmt.Errorf("no alert recipients — set alert emails on the target, customer notifications, or SMTP Alert Recipients")
			a.recordEmailSkip(logMeta, "", subject, emailErr.Error())
			log.Printf("alerter: email %s for %s: %v", meta.Event, t.Name, emailErr)
		} else {
			body := a.renderAlertEmail(meta)
			for _, to := range recipients {
				if err := a.sendSMTP(to, subject, body, logMeta); err != nil {
					emailErr = err
					log.Printf("alerter: email %s for %s: %v", meta.Event, t.Name, err)
					break
				}
			}
		}
	}

	a.fireSlackForAlert(t.TenantID, meta)
	a.fireWebhooks(meta.Event, payload)
	return emailErr
}

func (a *Alerter) perfRecipients(t *models.PerformanceTarget) []string {
	if t == nil {
		return a.platformAlertEmails()
	}
	return a.resolveAlertEmails(t.AlertEmails, t.TenantID)
}
