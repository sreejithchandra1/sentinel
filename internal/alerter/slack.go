package alerter

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *Alerter) fireSlackForAlert(tenantID string, meta AlertMeta) {
	tenantID = strings.TrimSpace(tenantID)
	a.fireSlack(tenantID, meta)
	if tenantID != "" {
		a.fireSlack("", meta)
	}
}

func (a *Alerter) fireSlack(tenantID string, meta AlertMeta) {
	if a.store == nil {
		return
	}
	cfg, err := a.store.GetSlackConfig(tenantID)
	if err != nil || !cfg.Enabled || strings.TrimSpace(cfg.WebhookURL) == "" {
		return
	}
	if !webhookMatchesEvent(cfg.Events, meta.Event) {
		return
	}
	body, err := buildSlackPayload(meta)
	if err != nil {
		log.Printf("slack: encode: %v", err)
		return
	}
	url := strings.TrimSpace(cfg.WebhookURL)
	if err := validateOutboundWebhookURL(url); err != nil {
		log.Printf("slack: blocked webhook url: %v", err)
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Printf("slack: request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := outboundHTTPClient(10 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("slack: FAIL: %v", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("slack: FAIL status=%d", resp.StatusCode)
			return
		}
		log.Printf("slack: SENT event=%s", meta.Event)
	}()
}

// SendTestSlack posts a test message using the given tenant's Slack config.
func (a *Alerter) SendTestSlack(tenantID string) error {
	cfg, err := a.store.GetSlackConfig(tenantID)
	if err != nil {
		return err
	}
	url := strings.TrimSpace(cfg.WebhookURL)
	if url == "" {
		return fmt.Errorf("Slack webhook URL is not configured")
	}
	if err := validateOutboundWebhookURL(url); err != nil {
		return err
	}
	meta := AlertMeta{
		Event:        "TEST",
		Name:         "Sentinel",
		URL:          a.liveDashboardURL(),
		Message:      "This is a test notification from Sentinel.",
		DashboardURL: a.liveDashboardURL(),
		EventAt:      time.Now().UTC(),
	}
	body, err := buildSlackPayload(meta)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Slack webhook test failed")
	}
	req.Header.Set("Content-Type", "application/json")
	client := outboundHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		// Do not return dial/status oracles (SSRF reconnaissance).
		return fmt.Errorf("Slack webhook test failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Slack webhook test failed")
	}
	return nil
}
