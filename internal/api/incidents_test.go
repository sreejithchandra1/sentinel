package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestIncidentErrorPageToken(t *testing.T) {
	srv, st, admin := newTestMFAServer(t)
	h := srv.Handler()
	adminSess := withSession(t, st, admin.ID)

	m := &models.Monitor{Name: "shop", URL: "https://shop.example", Enabled: true}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	inc := &models.Incident{
		MonitorID: m.ID,
		Type:      models.IncidentDown,
		Message:   "expected status 200, got 503 (Shopware maintenance)",
		StartedAt: time.Now().UTC(),
		ErrorPage: &models.HTTPErrorPage{
			StatusCode:  503,
			Source:      models.HTTPErrorSourceShopware,
			SourceLabel: "Shopware maintenance",
			BodyHTML:    "<html><head><title>Maintenance</title></head><body>Shopware down</body></html>",
			PageURL:     "https://shop.example/foo",
			Headers:     map[string]string{"Server": "nginx"},
		},
	}
	if err := st.CreateIncident(inc); err != nil {
		t.Fatal(err)
	}
	if inc.ErrorPage.ViewToken == "" {
		t.Fatal("expected view token")
	}

	unauth := httptest.NewRequest(http.MethodGet, "/api/incidents/"+inc.ID+"/error-page?token="+inc.ErrorPage.ViewToken, nil)
	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, unauth)
	if okRec.Code != http.StatusOK {
		t.Fatalf("valid token status=%d body=%s", okRec.Code, okRec.Body.String())
	}
	if ct := okRec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type=%s", ct)
	}
	if !strings.Contains(okRec.Body.String(), "Shopware down") {
		t.Fatalf("body=%s", okRec.Body.String())
	}
	if !strings.Contains(okRec.Body.String(), `<base href="https://shop.example/`) {
		t.Fatalf("missing base href: %s", okRec.Body.String())
	}
	if okRec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}

	bad := httptest.NewRequest(http.MethodGet, "/api/incidents/"+inc.ID+"/error-page?token=nope", nil)
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("bad token status=%d", badRec.Code)
	}

	get := authedReq(t, h, http.MethodGet, "/api/incidents/"+inc.ID, adminSess)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	var item models.IncidentListItem
	if err := json.Unmarshal(get.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ErrorPage == nil || item.ErrorPage.BodyHTML == "" {
		t.Fatalf("error_page=%+v", item.ErrorPage)
	}
	if item.ErrorPage.ViewToken != "" {
		t.Fatal("view token must not be returned on authenticated GET")
	}
	if item.ErrorPage.ViewURL == "" || !strings.Contains(item.ErrorPage.ViewURL, "error-page?token=") {
		t.Fatalf("view_url=%q", item.ErrorPage.ViewURL)
	}
	if strings.Contains(item.Message, "nginx error page") || strings.Contains(item.Message, "Shopware maintenance") {
		t.Fatalf("message should not include guessed source: %q", item.Message)
	}
	if item.Message != "expected status 200, got 503" {
		t.Fatalf("message=%q", item.Message)
	}
}
