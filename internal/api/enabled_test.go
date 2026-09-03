package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func authedJSON(t *testing.T, handler http.Handler, method, path, sessionID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: sessionID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestSetMonitorEnabled_PauseResumeAndAuth(t *testing.T) {
	srv, st, admin := newTestMFAServer(t)
	h := srv.Handler()
	adminSess := withSession(t, st, admin.ID)

	cust, err := st.CreateCustomer("Acme", 10)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateCustomer("Other", 10)
	if err != nil {
		t.Fatal(err)
	}
	m := &models.Monitor{Name: "api", URL: "https://api.example.com", Enabled: true, TenantID: cust.ID}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateMonitorState(m.ID, models.StatusDown, 2, m.UpdatedAt); err != nil {
		t.Fatal(err)
	}

	pause := authedJSON(t, h, http.MethodPut, "/api/monitors/"+m.ID+"/enabled", adminSess, map[string]bool{"enabled": false})
	if pause.Code != http.StatusOK {
		t.Fatalf("pause status=%d body=%s", pause.Code, pause.Body.String())
	}
	var got models.Monitor
	if err := json.Unmarshal(pause.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.Name != "api" || got.LastStatus != models.StatusDown {
		t.Fatalf("pause response: %+v", got)
	}

	stored, err := st.GetMonitor(m.ID)
	if err != nil || stored == nil || stored.Enabled || stored.URL != "https://api.example.com" {
		t.Fatalf("stored after pause: %v %v", stored, err)
	}

	resume := authedJSON(t, h, http.MethodPut, "/api/monitors/"+m.ID+"/enabled", adminSess, map[string]bool{"enabled": true})
	if resume.Code != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resume.Code, resume.Body.String())
	}
	if err := json.Unmarshal(resume.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled {
		t.Fatal("expected resumed")
	}

	missing := authedJSON(t, h, http.MethodPut, "/api/monitors/"+m.ID+"/enabled", adminSess, map[string]string{})
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing enabled status=%d", missing.Code)
	}

	viewer, err := st.CreateUser("viewer", "v@example.com", "Admin1234", models.RoleViewer, "")
	if err != nil {
		t.Fatal(err)
	}
	viewerSess := withSession(t, st, viewer.ID)
	forbidden := authedJSON(t, h, http.MethodPut, "/api/monitors/"+m.ID+"/enabled", viewerSess, map[string]bool{"enabled": false})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("viewer status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}

	tenantAdmin, err := st.CreateUser("tenantadmin", "t@example.com", "Admin1234", models.RoleAdmin, cust.ID)
	if err != nil {
		t.Fatal(err)
	}
	tenantSess := withSession(t, st, tenantAdmin.ID)
	own := authedJSON(t, h, http.MethodPut, "/api/monitors/"+m.ID+"/enabled", tenantSess, map[string]bool{"enabled": false})
	if own.Code != http.StatusOK {
		t.Fatalf("tenant pause status=%d body=%s", own.Code, own.Body.String())
	}

	otherMon := &models.Monitor{Name: "other", URL: "https://other.example.com", Enabled: true, TenantID: other.ID}
	if err := st.CreateMonitor(otherMon); err != nil {
		t.Fatal(err)
	}
	hidden := authedJSON(t, h, http.MethodPut, "/api/monitors/"+otherMon.ID+"/enabled", tenantSess, map[string]bool{"enabled": false})
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s", hidden.Code, hidden.Body.String())
	}
}

func TestSetPerformanceTargetEnabled_PauseResume(t *testing.T) {
	srv, st, admin := newTestMFAServer(t)
	h := srv.Handler()
	adminSess := withSession(t, st, admin.ID)

	target := &models.PerformanceTarget{Name: "lat", URL: "https://lat.example.com", Enabled: true, SlowThresholdMs: 900}
	if err := st.CreatePerformanceTarget(target); err != nil {
		t.Fatal(err)
	}

	pause := authedJSON(t, h, http.MethodPut, "/api/performance/targets/"+target.ID+"/enabled", adminSess, map[string]bool{"enabled": false})
	if pause.Code != http.StatusOK {
		t.Fatalf("pause status=%d body=%s", pause.Code, pause.Body.String())
	}
	var got models.PerformanceTarget
	if err := json.Unmarshal(pause.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.Name != "lat" || got.SlowThresholdMs != 900 {
		t.Fatalf("pause response: %+v", got)
	}

	resume := authedJSON(t, h, http.MethodPut, "/api/performance/targets/"+target.ID+"/enabled", adminSess, map[string]bool{"enabled": true})
	if resume.Code != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resume.Code, resume.Body.String())
	}
}
