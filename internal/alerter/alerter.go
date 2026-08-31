package alerter

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/config"
	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

const defaultFlapThreshold = 2

func monitorAlertAfterFailures(m *models.Monitor) int {
	if m.AlertAfterFailures < 1 {
		return defaultFlapThreshold
	}
	return m.AlertAfterFailures
}

type Alerter struct {
	store        *store.Store
	cfg          models.SMTPConfig
	fallbackSMTP models.SMTPConfig
	dashboardURL string

	// recoveryStreak counts consecutive non-down checks while a DOWN incident is open.
	// Recovery email fires only after reaching the same threshold used for DOWN alerts,
	// so brief flaps during a long outage do not spam DOWN/RECOVERY pairs.
	mu             sync.Mutex
	recoveryStreak map[string]int

	// notifyHook, when set, replaces NotifyMonitor (tests).
	notifyHook func(m *models.Monitor, alertType, message string, responseMs int) error
}

func New(s *store.Store, cfg models.SMTPConfig, fallback models.SMTPConfig, dashboardURL string) *Alerter {
	a := &Alerter{
		store:          s,
		cfg:            cfg,
		fallbackSMTP:   fallback,
		dashboardURL:   dashboardURL,
		recoveryStreak: make(map[string]int),
	}
	a.refreshSMTP()
	return a
}

func (a *Alerter) notifyMonitorAlert(m *models.Monitor, meta AlertMeta) error {
	if a.notifyHook != nil {
		return a.notifyHook(m, meta.Event, meta.Message, meta.ResponseMs)
	}
	return a.NotifyMonitorMeta(m, meta)
}

func (a *Alerter) incRecoveryStreak(monitorID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recoveryStreak == nil {
		a.recoveryStreak = make(map[string]int)
	}
	a.recoveryStreak[monitorID]++
	return a.recoveryStreak[monitorID]
}

func (a *Alerter) clearRecoveryStreak(monitorID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recoveryStreak != nil {
		delete(a.recoveryStreak, monitorID)
	}
}

func (a *Alerter) UpdateSMTP(cfg models.SMTPConfig) {
	a.cfg = cfg
}

func (a *Alerter) liveDashboardURL() string {
	base := strings.TrimRight(strings.TrimSpace(a.dashboardURL), "/")
	if a.store == nil {
		return base
	}
	cfg, err := a.store.GetServerSettings(config.ServerConfig{DashboardURL: base})
	if err != nil {
		return base
	}
	if u := strings.TrimRight(strings.TrimSpace(cfg.DashboardURL), "/"); u != "" {
		return u
	}
	return base
}

func (a *Alerter) refreshSMTP() {
	if a.store == nil {
		return
	}
	cfg, err := a.store.GetSMTPConfig(a.fallbackSMTP)
	if err != nil {
		log.Printf("alerter: load smtp config: %v", err)
		return
	}
	if cfg.Host != "" {
		a.cfg = cfg
	}
}

func (a *Alerter) HandleResult(m *models.Monitor, result *models.CheckResult) error {
	newStatus := result.Status
	failures := m.ConsecutiveFailures
	threshold := monitorAlertAfterFailures(m)

	open, err := a.store.GetOpenIncident(m.ID, models.IncidentDown)
	if err != nil {
		return err
	}

	// SSL expiry "critical" uses StatusDown for UI, but it is not a connectivity outage.
	if m.Type == models.MonitorSSL && newStatus == models.StatusDown {
		a.clearRecoveryStreak(m.ID)
		if err := a.store.UpdateMonitorState(m.ID, models.StatusDown, 0, result.CheckedAt); err != nil {
			return err
		}
		m.LastStatus = models.StatusDown
		m.ConsecutiveFailures = 0
		return nil
	}

	if newStatus == models.StatusDown {
		failures++
		a.clearRecoveryStreak(m.ID)

		if err := a.store.UpdateMonitorState(m.ID, models.StatusDown, failures, result.CheckedAt); err != nil {
			return err
		}
		m.LastStatus = models.StatusDown
		m.ConsecutiveFailures = failures

		// Exactly one DOWN email per outage (while the incident stays open).
		if failures < threshold {
			return nil
		}
		if open != nil {
			log.Printf("alerter: down alert skipped for %s: incident already open", m.Name)
			return nil
		}
		inc := &models.Incident{
			MonitorID: m.ID,
			Type:      models.IncidentDown,
			Message:   result.Error,
			StartedAt: result.CheckedAt,
		}
		if err := a.store.CreateIncident(inc); err != nil {
			return err
		}
		return a.notifyMonitorAlert(m, AlertMeta{
			Event:      "DOWN",
			Message:    result.Error,
			ResponseMs: result.ResponseTimeMs,
			IncidentID: inc.ID,
			EventAt:    result.CheckedAt,
		})
	}

	// Non-down check
	if open != nil {
		streak := a.incRecoveryStreak(m.ID)
		if streak < threshold {
			// Brief up during an outage: keep showing Down and do not email yet.
			if err := a.store.UpdateMonitorState(m.ID, models.StatusDown, m.ConsecutiveFailures, result.CheckedAt); err != nil {
				return err
			}
			m.LastStatus = models.StatusDown
			log.Printf("alerter: recovery pending for %s: %d/%d successful checks", m.Name, streak, threshold)
			return nil
		}
		a.clearRecoveryStreak(m.ID)
		if err := a.store.UpdateMonitorState(m.ID, newStatus, 0, result.CheckedAt); err != nil {
			return err
		}
		m.LastStatus = newStatus
		m.ConsecutiveFailures = 0
		started := open.StartedAt
		incidentID := open.ID
		_ = a.store.ResolveOpenIncidents(m.ID, models.IncidentDown, result.CheckedAt)
		_ = a.store.ResolveOpenIncidents(m.ID, models.IncidentSlow, result.CheckedAt)
		return a.notifyMonitorAlert(m, AlertMeta{
			Event:      "RECOVERY",
			Message:    "Monitor is back online",
			ResponseMs: result.ResponseTimeMs,
			IncidentID: incidentID,
			EventAt:    result.CheckedAt,
			StartedAt:  &started,
		})
	}

	a.clearRecoveryStreak(m.ID)
	if err := a.store.UpdateMonitorState(m.ID, newStatus, 0, result.CheckedAt); err != nil {
		return err
	}
	m.LastStatus = newStatus
	m.ConsecutiveFailures = 0

	// Uptime monitors do not email on latency/degraded — performance-target only.
	if newStatus != models.StatusDegraded {
		_ = a.store.ResolveOpenIncidents(m.ID, models.IncidentSlow, result.CheckedAt)
	}
	return nil
}

func (a *Alerter) SendPasswordResetEmail(to, username, resetURL string) error {
	if a.cfg.Host == "" {
		return fmt.Errorf("SMTP not configured — configure email in Settings")
	}
	if to == "" {
		return fmt.Errorf("no email address")
	}
	subject := "[Sentinel] Password Reset"
	body := a.renderPasswordResetEmail(username, resetURL)
	return a.sendSMTP(to, subject, body)
}

func (a *Alerter) SendPasswordChangedEmail(to, username string) error {
	if a.cfg.Host == "" || to == "" {
		return nil
	}
	subject := "[Sentinel] Password Changed"
	body := a.renderPasswordChangedEmail(username)
	return a.sendSMTP(to, subject, body)
}

func (a *Alerter) SendMFACodeEmail(to, username, code string) error {
	if a.cfg.Host == "" {
		return fmt.Errorf("SMTP not configured — configure email in Settings")
	}
	if to == "" {
		return fmt.Errorf("no email address")
	}
	subject := "[Sentinel] Your Verification Code"
	body := a.renderMFACodeEmail(username, code)
	return a.sendSMTP(to, subject, body)
}

func (a *Alerter) SendTestEmail(to string) error {
	if a.cfg.Host == "" {
		return fmt.Errorf("SMTP not configured")
	}
	if to == "" {
		to = a.cfg.From
	}
	subject := "[Sentinel] Test Email"
	body := a.renderAlertEmail(AlertMeta{
		Event:        "TEST",
		Name:         "Test Alert",
		URL:          a.liveDashboardURL(),
		Message:      "This is a test email from Sentinel.",
		DashboardURL: a.liveDashboardURL(),
		EventAt:      time.Now().UTC(),
	})
	return a.sendSMTP(to, subject, body)
}

func (a *Alerter) sendAlertMeta(m *models.Monitor, meta AlertMeta) error {
	a.refreshSMTP()
	if !a.cfg.Enabled {
		return fmt.Errorf("email alerts disabled")
	}
	if a.cfg.Host == "" {
		return fmt.Errorf("SMTP not configured — set host in Settings → Notifications → Email")
	}
	recipients := a.recipients(m)
	if len(recipients) == 0 {
		return fmt.Errorf("no alert recipients — set alert emails on the monitor, SMTP Alert Recipients, or a profile email")
	}
	subject := meta.FallbackText()
	body := a.renderAlertEmail(meta)
	for _, to := range recipients {
		if err := a.sendSMTP(to, subject, body); err != nil {
			return err
		}
	}
	return nil
}

func parseEmails(raw string) []string {
	var out []string
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func (a *Alerter) defaultRecipients() []string {
	if emails := parseEmails(a.cfg.AlertEmails); len(emails) > 0 {
		return emails
	}
	if a.store != nil {
		if emails, err := a.store.ListAlertProfileEmails(); err == nil && len(emails) > 0 {
			return emails
		}
	}
	return nil
}

func (a *Alerter) recipients(m *models.Monitor) []string {
	if emails := parseEmails(m.AlertEmails); len(emails) > 0 {
		return emails
	}
	return a.defaultRecipients()
}

func (a *Alerter) sendSMTP(to, subject, htmlBody string) error {
	a.refreshSMTP()
	if a.cfg.Host == "" {
		err := fmt.Errorf("SMTP not configured")
		log.Printf("smtp: SKIP to=%s subject=%q: %v", to, subject, err)
		return err
	}
	if to == "" {
		err := fmt.Errorf("empty recipient")
		log.Printf("smtp: SKIP subject=%q: %v", subject, err)
		return err
	}

	addr := fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port)
	from := a.cfg.From
	if from == "" {
		from = a.cfg.Username
	}
	if from == "" {
		err := fmt.Errorf("SMTP from address not configured")
		log.Printf("smtp: SKIP to=%s subject=%q: %v", to, subject, err)
		return err
	}

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", formatFromHeader(from)))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(htmlBody)

	auth := smtpAuth(a.cfg)

	var sendErr error
	if a.cfg.Port == 465 {
		sendErr = a.sendSMTPSImplicitTLS(addr, auth, from, to, msg.Bytes())
	} else {
		sendErr = a.sendSMTPStartTLS(addr, auth, from, to, msg.Bytes())
	}
	if sendErr != nil {
		log.Printf("smtp: FAIL to=%s from=%s subject=%q host=%s: %v", to, from, subject, addr, sendErr)
		return sendErr
	}
	log.Printf("smtp: SENT to=%s from=%s subject=%q host=%s", to, from, subject, addr)
	return nil
}

func smtpAuth(cfg models.SMTPConfig) smtp.Auth {
	if cfg.Username == "" {
		return nil
	}
	return smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
}

// formatFromHeader sets a friendly display name for inbox UIs while keeping the
// envelope address as the bare email (MAIL FROM).
func formatFromHeader(from string) string {
	from = strings.TrimSpace(from)
	if from == "" {
		return from
	}
	// Already has a display name.
	if strings.Contains(from, "<") && strings.Contains(from, ">") {
		return from
	}
	return fmt.Sprintf("\"Sentinel Monitoring\" <%s>", from)
}

func (a *Alerter) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: a.cfg.Host,
		MinVersion: tls.VersionTLS12,
	}
}

func (a *Alerter) sendSMTPStartTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(a.tlsConfig()); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	} else if a.cfg.TLS {
		return fmt.Errorf("server does not support STARTTLS")
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return client.Quit()
}

func (a *Alerter) sendSMTPSImplicitTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, a.tlsConfig())
	if err != nil {
		return fmt.Errorf("TLS connect: %w", err)
	}
	defer conn.Close()

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = a.cfg.Host
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return client.Quit()
}

var emailTmpl = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
</head>
<body style="margin:0;padding:0;background:#eef0f3;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#eef0f3;border-collapse:collapse;">
    <tr>
      <td align="center" style="padding:28px 12px;">
        <table role="presentation" width="560" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:560px;border-collapse:collapse;">
          <tr>
            <td style="background:#1a1f26;border-radius:12px;border:1px solid #2a313c;overflow:hidden;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="border-collapse:collapse;">
                <tr>
                  <td style="border-left:4px solid {{.Color}};padding:0;">
                    <div style="padding:20px 24px 8px;">
                      <div style="font-size:11px;letter-spacing:0.08em;color:#8b95a5;text-transform:uppercase;margin-bottom:8px;">Sentinel</div>
                      <div style="font-size:20px;font-weight:700;color:#f4f7fb;letter-spacing:0.02em;">{{.Title}}</div>
                    </div>
                    <div style="padding:8px 24px 20px;">
                      <div style="font-size:16px;font-weight:600;color:#ffffff;margin-bottom:4px;">{{.Name}}</div>
                      {{if .URL}}<div style="margin-bottom:16px;"><a href="{{.URL}}" style="color:#5b9fd4;text-decoration:none;font-size:13px;word-break:break-all;">{{.URL}}</a></div>{{end}}
                      {{if .ShowMessage}}<div style="color:#a8b3c2;font-size:13px;margin-bottom:16px;">{{.Message}}</div>{{end}}
                      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="width:100%;border-collapse:collapse;background:#12171e;border-radius:8px;">
                        <tr>
                          <td style="padding:12px 14px;width:50%;vertical-align:top;border-bottom:1px solid #243041;">
                            <div style="font-size:11px;color:#8b95a5;margin-bottom:4px;">{{.Field1Label}}</div>
                            <div style="font-size:14px;font-weight:600;color:{{.Field1Color}};">{{.Field1Value}}</div>
                          </td>
                          <td style="padding:12px 14px;width:50%;vertical-align:top;border-bottom:1px solid #243041;">
                            <div style="font-size:11px;color:#8b95a5;margin-bottom:4px;">Response</div>
                            <div style="font-size:14px;font-weight:600;color:#f4f7fb;">{{.Response}}</div>
                          </td>
                        </tr>
                        <tr>
                          <td style="padding:12px 14px;width:50%;vertical-align:top;">
                            <div style="font-size:11px;color:#8b95a5;margin-bottom:4px;">{{.TimeLabel}}</div>
                            <div style="font-size:14px;font-weight:600;color:#f4f7fb;">{{.TimeValue}}</div>
                          </td>
                          <td style="padding:12px 14px;width:50%;vertical-align:top;">
                            <div style="font-size:11px;color:#8b95a5;margin-bottom:4px;">Incident</div>
                            <div style="font-size:14px;font-weight:600;color:#f4f7fb;">{{.Incident}}</div>
                          </td>
                        </tr>
                      </table>
                      <div style="margin-top:20px;">
                        <a href="{{.DashboardURL}}" style="display:inline-block;background:#2B7A78;color:#ffffff;padding:10px 16px;border-radius:8px;text-decoration:none;font-size:13px;font-weight:600;">Open in Sentinel →</a>
                      </div>
                    </div>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="text-align:center;color:#6b7280;font-size:11px;padding-top:14px;">Sentinel Infrastructure Monitoring</td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

type emailData struct {
	Title, Name, URL, Message, DashboardURL      string
	Color, Field1Label, Field1Value, Field1Color string
	Response, TimeLabel, TimeValue, Incident     string
	ShowMessage                                  bool
}

func (a *Alerter) renderAlertEmail(meta AlertMeta) string {
	field1Label, field1Value := "Status", meta.StatusLabel()
	field1Color := meta.Color()
	if dt := meta.DowntimeLabel(); dt != "" {
		field1Label = "Downtime"
		field1Value = dt
		field1Color = "#f4f7fb"
	}
	showMsg := meta.Message != "" && meta.Event != "RECOVERY" && meta.Event != "NORMAL" && meta.ResponseLabel() != "Timeout"
	var buf bytes.Buffer
	_ = emailTmpl.Execute(&buf, emailData{
		Title:        meta.Title(),
		Name:         meta.Name,
		URL:          meta.URL,
		Message:      meta.Message,
		ShowMessage:  showMsg,
		DashboardURL: meta.DashboardURL,
		Color:        meta.Color(),
		Field1Label:  field1Label,
		Field1Value:  field1Value,
		Field1Color:  field1Color,
		Response:     meta.ResponseLabel(),
		TimeLabel:    meta.TimeFieldLabel(),
		TimeValue:    meta.EventTimeLabel(),
		Incident:     meta.IncidentLabel(),
	})
	return buf.String()
}

func (a *Alerter) renderEmail(name, url, dashboardURL, message, alertType string, responseMs int) string {
	return a.renderAlertEmail(AlertMeta{
		Event:        alertType,
		Name:         name,
		URL:          url,
		Message:      message,
		DashboardURL: dashboardURL,
		ResponseMs:   responseMs,
		EventAt:      time.Now().UTC(),
	})
}

var resetEmailTmpl = template.Must(template.New("reset").Parse(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;background:#f4f4f5;padding:20px;">
<div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;border:1px solid #e4e4e7;">
  <h2 style="margin:0 0 8px;color:#18181b;">Reset your password</h2>
  <p style="color:#71717a;">Hi {{.Username}}, click the button below to set a new password. This link expires in 1 hour.</p>
  <p style="margin-top:20px;"><a href="{{.ResetURL}}" style="background:#2B7A78;color:#fff;padding:10px 16px;border-radius:6px;text-decoration:none;">Reset Password</a></p>
  <p style="color:#71717a;font-size:12px;margin-top:24px;">If you did not request this, you can ignore this email.</p>
</div></body></html>`))

func (a *Alerter) renderPasswordResetEmail(username, resetURL string) string {
	var buf bytes.Buffer
	_ = resetEmailTmpl.Execute(&buf, map[string]string{"Username": username, "ResetURL": resetURL})
	return buf.String()
}

var passwordChangedTmpl = template.Must(template.New("changed").Parse(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;background:#f4f4f5;padding:20px;">
<div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;border:1px solid #e4e4e7;">
  <h2 style="margin:0 0 8px;color:#18181b;">Password changed successfully</h2>
  <p style="color:#71717a;">Hi {{.Username}}, your Sentinel account password was changed on {{.Time}}.</p>
  <p style="color:#71717a;">If you made this change, no further action is needed.</p>
  <p style="color:#71717a;font-size:13px;margin-top:20px;">If you did <strong>not</strong> change your password, contact your administrator immediately and sign in to update your credentials.</p>
  <p style="margin-top:20px;"><a href="{{.DashboardURL}}" style="background:#2B7A78;color:#fff;padding:10px 16px;border-radius:6px;text-decoration:none;">Open Sentinel</a></p>
</div></body></html>`))

func (a *Alerter) renderPasswordChangedEmail(username string) string {
	var buf bytes.Buffer
	_ = passwordChangedTmpl.Execute(&buf, map[string]string{
		"Username":     username,
		"Time":         time.Now().UTC().Format(time.RFC1123),
		"DashboardURL": a.liveDashboardURL() + "/login",
	})
	return buf.String()
}

var mfaCodeTmpl = template.Must(template.New("mfa").Parse(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;background:#f4f4f5;padding:20px;">
<div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;border:1px solid #e4e4e7;">
  <h2 style="margin:0 0 8px;color:#18181b;">Your sign-in verification code</h2>
  <p style="color:#71717a;">Hi {{.Username}}, use the code below to finish signing in to Sentinel. This code expires in 10 minutes.</p>
  <div style="margin:24px 0;padding:16px 20px;border-radius:8px;background:#0f172a;color:#f8fafc;font-size:28px;font-weight:700;letter-spacing:0.3em;text-align:center;">{{.Code}}</div>
  <p style="color:#71717a;">If you did not try to sign in, you can ignore this email.</p>
</div></body></html>`))

func (a *Alerter) renderMFACodeEmail(username, code string) string {
	var buf bytes.Buffer
	_ = mfaCodeTmpl.Execute(&buf, map[string]string{
		"Username": username,
		"Code":     code,
	})
	return buf.String()
}
