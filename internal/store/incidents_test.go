package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestAcknowledgeIncident(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{Name: "web", URL: "https://example.com", Enabled: true}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	inc := &models.Incident{
		MonitorID: m.ID,
		Type:      models.IncidentDown,
		Message:   "timeout",
		StartedAt: started,
	}
	if err := st.CreateIncident(inc); err != nil {
		t.Fatal(err)
	}

	at := started.Add(5 * time.Minute)
	got, err := st.AcknowledgeIncident(inc.ID, "Ada", at)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.AcknowledgedBy != "Ada" || got.AcknowledgedAt == nil {
		t.Fatalf("ack missing: %+v", got)
	}
	if !got.AcknowledgedAt.Equal(at) {
		t.Fatalf("acked at %v, want %v", got.AcknowledgedAt, at)
	}
	if got.ResolvedAt != nil {
		t.Fatal("ack must not resolve")
	}

	again, err := st.AcknowledgeIncident(inc.ID, "Bob", at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if again.AcknowledgedBy != "Ada" {
		t.Fatalf("second ack should be no-op, got %q", again.AcknowledgedBy)
	}

	open, err := st.GetOpenIncident(m.ID, models.IncidentDown)
	if err != nil || open == nil {
		t.Fatalf("incident should stay open: %v %v", open, err)
	}

	resolvedAt := at.Add(10 * time.Minute)
	if err := st.ResolveIncident(inc.ID, resolvedAt); err != nil {
		t.Fatal(err)
	}
	_, err = st.AcknowledgeIncident(inc.ID, "Ada", resolvedAt.Add(time.Minute))
	if !errors.Is(err, ErrIncidentResolved) {
		t.Fatalf("want ErrIncidentResolved, got %v", err)
	}

	listed, err := st.QueryIncidents(IncidentQuery{MonitorID: m.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].AcknowledgedBy != "Ada" || listed[0].ResolvedAt == nil {
		t.Fatalf("list: %+v", listed)
	}
}

func TestPruneOldIncidentCaptures(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{Name: "shop", URL: "https://shop.example", Enabled: true}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	page := &models.HTTPErrorPage{
		StatusCode: 503,
		BodyHTML:   "<html>maintenance</html>",
		PageURL:    "https://shop.example/",
	}

	oldResolvedAt := now.AddDate(0, 0, -40)
	oldResolved := &models.Incident{
		MonitorID:  m.ID,
		Type:       models.IncidentDown,
		Message:    "expected status 200, got 503",
		StartedAt:  oldResolvedAt.Add(-time.Hour),
		ResolvedAt: &oldResolvedAt,
		ErrorPage:  page,
	}
	if err := st.CreateIncident(oldResolved); err != nil {
		t.Fatal(err)
	}

	recentResolvedAt := now.AddDate(0, 0, -2)
	recentResolved := &models.Incident{
		MonitorID:  m.ID,
		Type:       models.IncidentDown,
		Message:    "expected status 200, got 503",
		StartedAt:  recentResolvedAt.Add(-time.Hour),
		ResolvedAt: &recentResolvedAt,
		ErrorPage:  page,
	}
	if err := st.CreateIncident(recentResolved); err != nil {
		t.Fatal(err)
	}

	oldOpen := &models.Incident{
		MonitorID: m.ID,
		Type:      models.IncidentDown,
		Message:   "expected status 200, got 503",
		StartedAt: now.AddDate(0, 0, -40),
		ErrorPage: page,
	}
	if err := st.CreateIncident(oldOpen); err != nil {
		t.Fatal(err)
	}

	n, err := st.PruneOldIncidentCaptures(now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned=%d want 1", n)
	}

	got, _, err := st.GetIncident(oldResolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ErrorPage != nil {
		t.Fatalf("old resolved capture should be gone: %+v", got.ErrorPage)
	}

	got, _, err = st.GetIncident(recentResolved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ErrorPage == nil || got.ErrorPage.BodyHTML == "" {
		t.Fatal("recent resolved capture should be kept")
	}

	got, _, err = st.GetIncident(oldOpen.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ErrorPage == nil || got.ErrorPage.BodyHTML == "" {
		t.Fatal("open incident capture should be kept")
	}
}
