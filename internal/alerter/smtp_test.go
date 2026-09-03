package alerter

import (
	"path/filepath"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestSendTestEmailsRequiresRecipients(t *testing.T) {
	a := &Alerter{cfg: models.SMTPConfig{Host: "smtp.example.com", From: "alerts@example.com"}}
	if err := a.SendTestEmails(""); err == nil {
		t.Fatal("expected error when no recipients")
	}
	if err := a.SendTestEmails("  ,  "); err == nil {
		t.Fatal("expected error when recipients are blank")
	}
}

func TestDefaultRecipientsUsesAlertEmails(t *testing.T) {
	a := &Alerter{cfg: models.SMTPConfig{AlertEmails: "ops@example.com", From: "alerts@example.com"}}
	got := a.defaultRecipients()
	if len(got) != 1 || got[0] != "ops@example.com" {
		t.Fatalf("defaultRecipients = %v, want ops@example.com", got)
	}
}

func TestDefaultRecipientsDoesNotUseFrom(t *testing.T) {
	a := &Alerter{cfg: models.SMTPConfig{From: "alerts@example.com"}}
	got := a.defaultRecipients()
	if len(got) != 0 {
		t.Fatalf("defaultRecipients = %v, want empty (From is sender only)", got)
	}
}

func TestRecipientsMonitorEmails(t *testing.T) {
	a := &Alerter{cfg: models.SMTPConfig{From: "from@example.com"}}
	m := &models.Monitor{AlertEmails: "a@x.com, b@x.com"}
	got := a.recipients(m)
	if len(got) != 2 {
		t.Fatalf("recipients = %v", got)
	}
}

func TestPerfRecipientsUsesMonitorThenDefault(t *testing.T) {
	a := &Alerter{cfg: models.SMTPConfig{AlertEmails: "ops@example.com", From: "alerts@example.com"}}
	t1 := &models.PerformanceTarget{AlertEmails: ""}
	got := a.perfRecipients(t1)
	if len(got) != 1 || got[0] != "ops@example.com" {
		t.Fatalf("perfRecipients = %v", got)
	}
	t2 := &models.PerformanceTarget{AlertEmails: "perf@example.com"}
	got = a.perfRecipients(t2)
	if len(got) != 1 || got[0] != "perf@example.com" {
		t.Fatalf("perfRecipients = %v", got)
	}
}

func TestRecipientsNeverUseUserProfiles(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.CreateUser("ops", "ops@users.example", "passw0rd1", models.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{From: "from@example.com"}, models.SMTPConfig{}, "http://localhost")
	got := a.recipients(&models.Monitor{})
	if len(got) != 0 {
		t.Fatalf("recipients = %v, want empty (no user-table fallback)", got)
	}
}

func TestRecipientsTenantDefaultPlusPlatformCC(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cust, err := st.CreateCustomer("Acme", 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateCustomer(cust.ID, cust.Name, cust.MonitorQuota, "cust@x.com, Other@x.com"); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{AlertEmails: "ops@example.com, other@x.com"}, models.SMTPConfig{}, "http://localhost")
	got := a.recipients(&models.Monitor{TenantID: cust.ID})
	if len(got) != 3 {
		t.Fatalf("recipients = %v, want customer list + platform CC (deduped)", got)
	}
	if got[0] != "cust@x.com" || got[1] != "Other@x.com" || got[2] != "ops@example.com" {
		t.Fatalf("recipients = %v", got)
	}
}

func TestRecipientsTenantOverridePlusPlatformCC(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cust, err := st.CreateCustomer("Acme", 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateCustomer(cust.ID, cust.Name, cust.MonitorQuota, "cust@x.com"); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{AlertEmails: "ops@example.com"}, models.SMTPConfig{}, "http://localhost")
	got := a.recipients(&models.Monitor{TenantID: cust.ID, AlertEmails: "override@x.com"})
	if len(got) != 2 || got[0] != "override@x.com" || got[1] != "ops@example.com" {
		t.Fatalf("recipients = %v, want override + platform CC, not customer default", got)
	}
	got = a.perfRecipients(&models.PerformanceTarget{TenantID: cust.ID, AlertEmails: "perf@x.com"})
	if len(got) != 2 || got[0] != "perf@x.com" || got[1] != "ops@example.com" {
		t.Fatalf("perfRecipients = %v", got)
	}
}

func TestRecipientsInternalUsesSMTPOnly(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cust, err := st.CreateCustomer("Acme", 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateCustomer(cust.ID, cust.Name, cust.MonitorQuota, "cust@x.com"); err != nil {
		t.Fatal(err)
	}
	a := New(st, models.SMTPConfig{AlertEmails: "ops@example.com"}, models.SMTPConfig{}, "http://localhost")
	got := a.recipients(&models.Monitor{TenantID: ""})
	if len(got) != 1 || got[0] != "ops@example.com" {
		t.Fatalf("internal recipients = %v, want SMTP list only", got)
	}
}
