package alerter

import (
	"fmt"
	"log"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type smtpLogMeta struct {
	Kind        string
	MonitorID   string
	MonitorName string
	TenantID    string
}

func alertLogMeta(m *models.Monitor) smtpLogMeta {
	meta := smtpLogMeta{Kind: models.EmailKindAlert}
	if m == nil {
		return meta
	}
	meta.MonitorID = m.ID
	meta.MonitorName = m.Name
	meta.TenantID = m.TenantID
	return meta
}

func perfLogMeta(t *models.PerformanceTarget) smtpLogMeta {
	meta := smtpLogMeta{Kind: models.EmailKindAlert}
	if t == nil {
		return meta
	}
	meta.MonitorID = t.ID
	meta.MonitorName = t.Name
	meta.TenantID = t.TenantID
	return meta
}

func (a *Alerter) recordEmail(e models.EmailLog) {
	if a == nil || a.store == nil {
		return
	}
	if e.Kind == "" {
		e.Kind = models.EmailKindAlert
	}
	if err := a.store.InsertEmailLog(&e); err != nil {
		log.Printf("alerter: email log: %v", err)
	}
}

func (a *Alerter) recordEmailSkip(meta smtpLogMeta, to, subject, errMsg string) {
	a.recordEmail(models.EmailLog{
		Status:      models.EmailStatusSkip,
		Kind:        meta.Kind,
		ToAddr:      to,
		Subject:     subject,
		Error:       errMsg,
		MonitorID:   meta.MonitorID,
		MonitorName: meta.MonitorName,
		TenantID:    meta.TenantID,
	})
}

func (a *Alerter) recordEmailPending(meta smtpLogMeta, subject, errMsg string) {
	a.recordEmail(models.EmailLog{
		Status:      models.EmailStatusPending,
		Kind:        meta.Kind,
		Subject:     subject,
		Error:       errMsg,
		MonitorID:   meta.MonitorID,
		MonitorName: meta.MonitorName,
		TenantID:    meta.TenantID,
	})
}

func downSubject(name string) string {
	return fmt.Sprintf("[Sentinel] DOWN: %s", name)
}

func recoverySubject(name string) string {
	return fmt.Sprintf("[Sentinel] RECOVERY: %s", name)
}

func slowSubject(name string) string {
	return fmt.Sprintf("[Sentinel] SLOW: %s", name)
}
