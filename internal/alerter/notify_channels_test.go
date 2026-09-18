package alerter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestNotifyMonitor_SlackSkippedWhenMonitorOptOut(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveSlackConfig("", models.SlackConfig{
		WebhookURL: srv.URL,
		Enabled:    true,
		Events:     []string{"all"},
	}); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Enabled: false}, models.SMTPConfig{}, "http://localhost")
	m := &models.Monitor{
		ID: "m1", Name: "Site", URL: "https://example.com", Type: models.MonitorHTTP,
		NotifyEmail: true, NotifySlack: false, NotifyWebhooks: true,
	}
	_ = a.NotifyMonitor(m, "DOWN", "unreachable", 50)
	time.Sleep(150 * time.Millisecond)
	if hits.Load() != 0 {
		t.Fatalf("slack hits=%d want 0 when notify_slack=false", hits.Load())
	}
}

func TestNotifyMonitor_SlackFiresWhenEmailDisabled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("empty body")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveSlackConfig("", models.SlackConfig{
		WebhookURL: srv.URL,
		Enabled:    true,
		Events:     []string{"all"},
	}); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Enabled: false}, models.SMTPConfig{}, "http://localhost")
	m := &models.Monitor{
		ID: "m1", Name: "Site", URL: "https://example.com", Type: models.MonitorHTTP,
		NotifyEmail: true, NotifySlack: true, NotifyWebhooks: true,
	}
	_ = a.NotifyMonitor(m, "DOWN", "unreachable", 50)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("slack hits=%d want 1", hits.Load())
	}
}

func TestNotifyMonitor_TenantSlackScoping(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveSlackConfig("tenant-a", models.SlackConfig{
		WebhookURL: srv.URL,
		Enabled:    true,
		Events:     []string{"all"},
	}); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Enabled: false}, models.SMTPConfig{}, "http://localhost")
	// Platform monitor should not hit tenant-a Slack.
	_ = a.NotifyMonitor(&models.Monitor{
		ID: "1", Name: "P", URL: "https://p.example", TenantID: "",
		NotifyEmail: true, NotifySlack: true, NotifyWebhooks: true,
	}, "DOWN", "x", 1)
	time.Sleep(100 * time.Millisecond)
	if hits.Load() != 0 {
		t.Fatalf("platform alert hit tenant slack: %d", hits.Load())
	}

	_ = a.NotifyMonitor(&models.Monitor{
		ID: "2", Name: "C", URL: "https://c.example", TenantID: "tenant-a",
		NotifyEmail: true, NotifySlack: true, NotifyWebhooks: true,
	}, "DOWN", "x", 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("tenant alert hits=%d want 1", hits.Load())
	}
}

func TestNotifyMonitor_TenantAlsoFiresPlatformSlack(t *testing.T) {
	var tenantHits, platformHits atomic.Int32
	tenantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer tenantSrv.Close()
	platformSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer platformSrv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveSlackConfig("tenant-a", models.SlackConfig{
		WebhookURL: tenantSrv.URL, Enabled: true, Events: []string{"all"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSlackConfig("", models.SlackConfig{
		WebhookURL: platformSrv.URL, Enabled: true, Events: []string{"all"},
	}); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Enabled: false}, models.SMTPConfig{}, "http://localhost")
	_ = a.NotifyMonitor(&models.Monitor{
		ID: "2", Name: "C", URL: "https://c.example", TenantID: "tenant-a",
		NotifyEmail: true, NotifySlack: true, NotifyWebhooks: true,
	}, "DOWN", "x", 1)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (tenantHits.Load() == 0 || platformHits.Load() == 0) {
		time.Sleep(20 * time.Millisecond)
	}
	if tenantHits.Load() != 1 || platformHits.Load() != 1 {
		t.Fatalf("tenant=%d platform=%d want 1 each", tenantHits.Load(), platformHits.Load())
	}

	_ = a.NotifyMonitor(&models.Monitor{
		ID: "1", Name: "P", URL: "https://p.example", TenantID: "",
		NotifyEmail: true, NotifySlack: true, NotifyWebhooks: true,
	}, "DOWN", "x", 1)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && platformHits.Load() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if tenantHits.Load() != 1 {
		t.Fatalf("platform alert must not hit tenant slack: tenant=%d", tenantHits.Load())
	}
	if platformHits.Load() != 2 {
		t.Fatalf("platformHits=%d want 2", platformHits.Load())
	}
}

func TestNotifyMonitorMeta_WebhookIncludesErrorPageURL(t *testing.T) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveWebhooks([]models.WebhookConfig{{
		URL: srv.URL, Enabled: true, Events: []string{"all"},
	}}); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Enabled: false}, models.SMTPConfig{}, "http://localhost")
	m := &models.Monitor{
		ID: "m1", Name: "Shop", URL: "https://shop.example", Type: models.MonitorHTTP,
		NotifyEmail: false, NotifySlack: false, NotifyWebhooks: true,
	}
	pageURL := "http://localhost/api/incidents/abc/error-page?token=secret"
	if err := a.NotifyMonitorMeta(m, AlertMeta{
		Event:        "DOWN",
		Message:      "expected status 200, got 503",
		ErrorPageURL: pageURL,
		IncidentID:   "abc",
		EventAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		if v, ok := body.Load().(string); ok && v != "" {
			got = v
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got == "" {
		t.Fatal("webhook was not called")
	}
	if !strings.Contains(got, pageURL) {
		t.Fatalf("missing error_page_url: %s", got)
	}
	if strings.Contains(got, "error_source") {
		t.Fatalf("webhook should not guess error source: %s", got)
	}
}
