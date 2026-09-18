package alerter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestHandleResult_OneDownEmailPerOutage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:               "site",
		Type:               models.MonitorHTTP,
		URL:                "https://example.com",
		IntervalSeconds:    60,
		TimeoutMs:          5000,
		SlowThresholdMs:    3000,
		Enabled:            true,
		AlertAfterFailures: 2,
		LastStatus:         models.StatusUp,
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}

	var alerts []string
	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(_ *models.Monitor, alertType, _ string, _ int) error {
		alerts = append(alerts, alertType)
		return nil
	}

	now := time.Now().UTC()
	down := func(i int) *models.CheckResult {
		return &models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusDown,
			Error:     "timeout",
			CheckedAt: now.Add(time.Duration(i) * time.Minute),
		}
	}
	up := func(i int) *models.CheckResult {
		return &models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusUp,
			CheckedAt: now.Add(time.Duration(i) * time.Minute),
		}
	}

	// Need 2 consecutive failures before DOWN.
	if err := a.HandleResult(m, down(1)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Fatalf("alerts=%v, want none before threshold", alerts)
	}

	if err := a.HandleResult(m, down(2)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0] != "DOWN" {
		t.Fatalf("alerts=%v, want [DOWN]", alerts)
	}

	// Long outage with more failures — no extra DOWN emails.
	for i := 3; i <= 20; i++ {
		if err := a.HandleResult(m, down(i)); err != nil {
			t.Fatal(err)
		}
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts=%v, want a single DOWN during outage", alerts)
	}

	// One brief up is not enough for RECOVERY (threshold=2).
	if err := a.HandleResult(m, up(21)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts=%v, want still only DOWN after one up", alerts)
	}
	if m.LastStatus != models.StatusDown {
		t.Fatalf("last_status=%s, want down while recovery pending", m.LastStatus)
	}

	// Flap back down — still no second DOWN email.
	if err := a.HandleResult(m, down(22)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts=%v, want still one DOWN after flap", alerts)
	}

	// Confirmed recovery: 2 consecutive ups.
	if err := a.HandleResult(m, up(23)); err != nil {
		t.Fatal(err)
	}
	if err := a.HandleResult(m, up(24)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || alerts[1] != "RECOVERY" {
		t.Fatalf("alerts=%v, want [DOWN RECOVERY]", alerts)
	}
	if m.LastStatus != models.StatusUp {
		t.Fatalf("last_status=%s, want up after recovery", m.LastStatus)
	}

	// A later outage may send DOWN again.
	if err := a.HandleResult(m, down(25)); err != nil {
		t.Fatal(err)
	}
	if err := a.HandleResult(m, down(26)); err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 3 || alerts[2] != "DOWN" {
		t.Fatalf("alerts=%v, want second outage DOWN", alerts)
	}
}

func TestHandleResult_StoresErrorPageOnDown(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:               "shop",
		Type:               models.MonitorHTTP,
		URL:                "https://shop.example",
		Enabled:            true,
		AlertAfterFailures: 2,
		LastStatus:         models.StatusUp,
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost:8082")
	a.notifyHook = func(_ *models.Monitor, _ string, _ string, _ int) error { return nil }

	page := &models.HTTPErrorPage{
		StatusCode:  503,
		Source:      models.HTTPErrorSourceShopware,
		SourceLabel: "Shopware maintenance",
		BodyHTML:    "<html><body>maintenance</body></html>",
		PageURL:     "https://shop.example/",
		Headers:     map[string]string{"Server": "nginx"},
	}
	now := time.Now().UTC()
	first := &models.CheckResult{
		MonitorID: m.ID, Status: models.StatusDown, Error: "expected status 200, got 503 (Shopware maintenance)",
		CheckedAt: now, ErrorPage: page,
	}
	if err := a.HandleResult(m, first); err != nil {
		t.Fatal(err)
	}
	open, err := st.GetOpenIncident(m.ID, models.IncidentDown)
	if err != nil {
		t.Fatal(err)
	}
	if open != nil {
		t.Fatal("should not open incident before flap threshold")
	}

	second := &models.CheckResult{
		MonitorID: m.ID, Status: models.StatusDown, Error: "expected status 200, got 503 (Shopware maintenance)",
		CheckedAt: now.Add(time.Minute), ErrorPage: page,
	}
	if err := a.HandleResult(m, second); err != nil {
		t.Fatal(err)
	}
	item, _, err := st.GetIncident((func() string {
		inc, err := st.GetOpenIncident(m.ID, models.IncidentDown)
		if err != nil || inc == nil {
			t.Fatalf("open incident: %v %v", inc, err)
		}
		return inc.ID
	})())
	if err != nil {
		t.Fatal(err)
	}
	if item.ErrorPage == nil || item.ErrorPage.BodyHTML == "" {
		t.Fatalf("expected captured page, got %+v", item.ErrorPage)
	}
	if item.ErrorPage.ViewToken == "" {
		t.Fatal("expected view token")
	}
	if item.ErrorPage.Source != models.HTTPErrorSourceShopware {
		t.Fatalf("source=%s", item.ErrorPage.Source)
	}

	timeout := &models.CheckResult{
		MonitorID: m.ID, Status: models.StatusDown, Error: "connection timed out (host unreachable)",
		CheckedAt: now.Add(24 * time.Hour),
	}
	m.LastStatus = models.StatusUp
	m.ConsecutiveFailures = 0
	_ = st.ResolveOpenIncidents(m.ID, models.IncidentDown, now.Add(2*time.Hour))
	if err := a.HandleResult(m, timeout); err != nil {
		t.Fatal(err)
	}
	if err := a.HandleResult(m, timeout); err != nil {
		t.Fatal(err)
	}
	open, err = st.GetOpenIncident(m.ID, models.IncidentDown)
	if err != nil || open == nil {
		t.Fatalf("timeout incident: %v %v", open, err)
	}
	timeoutItem, _, err := st.GetIncident(open.ID)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutItem.ErrorPage != nil {
		t.Fatalf("timeout should not store error page, got %+v", timeoutItem.ErrorPage)
	}
}
