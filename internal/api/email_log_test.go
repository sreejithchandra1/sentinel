package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestListEmailLogAuth(t *testing.T) {
	srv, st, admin := newTestMFAServer(t)
	h := srv.Handler()

	if err := st.InsertEmailLog(&models.EmailLog{
		Status:  models.EmailStatusSent,
		Kind:    models.EmailKindTest,
		ToAddr:  "ops@example.com",
		Subject: "test",
	}); err != nil {
		t.Fatal(err)
	}

	bare := httptest.NewRequest(http.MethodGet, "/api/settings/smtp/log", nil)
	bareRec := httptest.NewRecorder()
	h.ServeHTTP(bareRec, bare)
	if bareRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d body=%s", bareRec.Code, bareRec.Body.String())
	}

	cust, err := st.CreateCustomer("Acme", 5)
	if err != nil {
		t.Fatal(err)
	}
	tenantAdmin, err := st.CreateUser("tenantadmin", "t@example.com", "Admin1234", models.RoleAdmin, cust.ID)
	if err != nil {
		t.Fatal(err)
	}
	tenantSess := withSession(t, st, tenantAdmin.ID)
	forbidden := authedReq(t, h, http.MethodGet, "/api/settings/smtp/log", tenantSess)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("customer admin status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}

	adminSess := withSession(t, st, admin.ID)
	ok := authedReq(t, h, http.MethodGet, "/api/settings/smtp/log", adminSess)
	if ok.Code != http.StatusOK {
		t.Fatalf("platform admin status=%d body=%s", ok.Code, ok.Body.String())
	}
	var page struct {
		Items []models.EmailLog `json:"items"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ToAddr != "ops@example.com" {
		t.Fatalf("page=%+v", page)
	}

	filtered := authedReq(t, h, http.MethodGet, "/api/settings/smtp/log?status=fail", adminSess)
	if filtered.Code != http.StatusOK {
		t.Fatalf("filter status=%d", filtered.Code)
	}
	if err := json.Unmarshal(filtered.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("fail filter total=%d want 0", page.Total)
	}
}
