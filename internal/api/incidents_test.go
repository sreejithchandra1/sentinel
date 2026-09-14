package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func withSession(t *testing.T, st interface {
	CreateSession(id, userID string, expiresAt time.Time) error
}, userID string) string {
	t.Helper()
	sid := "sess-" + userID[:8]
	if err := st.CreateSession(sid, userID, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return sid
}

func authedReq(t *testing.T, handler http.Handler, method, path, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: sessionID, HttpOnly: true, Secure: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGetAndAcknowledgeIncident(t *testing.T) {
	srv, st, admin := newTestMFAServer(t)
	h := srv.Handler()
	adminSess := withSession(t, st, admin.ID)

	cust, err := st.CreateCustomer("Acme", 10)
	if err != nil {
		t.Fatal(err)
	}

	m := &models.Monitor{Name: "api", URL: "https://api.example.com", Enabled: true, TenantID: cust.ID}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	inc := &models.Incident{
		MonitorID: m.ID,
		Type:      models.IncidentDown,
		Message:   "connection refused",
		StartedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := st.CreateIncident(inc); err != nil {
		t.Fatal(err)
	}

	get := authedReq(t, h, http.MethodGet, "/api/incidents/"+inc.ID, adminSess)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	var item models.IncidentListItem
	if err := json.Unmarshal(get.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.MonitorName != "api" || item.AcknowledgedAt != nil {
		t.Fatalf("GET item: %+v", item)
	}

	ack := authedReq(t, h, http.MethodPost, "/api/incidents/"+inc.ID+"/acknowledge", adminSess)
	if ack.Code != http.StatusOK {
		t.Fatalf("ACK status=%d body=%s", ack.Code, ack.Body.String())
	}
	if err := json.Unmarshal(ack.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.AcknowledgedBy == "" || item.AcknowledgedAt == nil || item.ResolvedAt != nil {
		t.Fatalf("ack fields: %+v", item)
	}
	if item.AcknowledgedBy != admin.Username && item.AcknowledgedBy != admin.Name {
		t.Fatalf("ack actor=%q", item.AcknowledgedBy)
	}

	again := authedReq(t, h, http.MethodPost, "/api/incidents/"+inc.ID+"/acknowledge", adminSess)
	if again.Code != http.StatusOK {
		t.Fatalf("second ack status=%d", again.Code)
	}

	if err := st.ResolveIncident(inc.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	resolved := authedReq(t, h, http.MethodPost, "/api/incidents/"+inc.ID+"/acknowledge", adminSess)
	if resolved.Code != http.StatusConflict {
		t.Fatalf("resolved ack status=%d body=%s", resolved.Code, resolved.Body.String())
	}

	other, err := st.CreateCustomer("Other", 10)
	if err != nil {
		t.Fatal(err)
	}
	tenantUser, err := st.CreateUser("otheradmin", "o@example.com", "Admin1234", models.RoleAdmin, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherSess := withSession(t, st, tenantUser.ID)
	hidden := authedReq(t, h, http.MethodGet, "/api/incidents/"+inc.ID, otherSess)
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant GET status=%d", hidden.Code)
	}

	missing := authedReq(t, h, http.MethodGet, "/api/incidents/does-not-exist", adminSess)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing GET status=%d", missing.Code)
	}
}
