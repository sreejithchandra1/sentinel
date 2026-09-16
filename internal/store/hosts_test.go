package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestHostCRUDAndEnroll(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := &models.Host{Name: "web-1", Enabled: true, NotifyEmail: true}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	if h.ID == "" || h.Status != models.HostPending {
		t.Fatalf("create: %+v", h)
	}
	if !h.CollectCPU || h.AlertCPUEnabled {
		t.Fatalf("defaults: collect=%v alert=%v", h.CollectCPU, h.AlertCPUEnabled)
	}

	got, err := st.GetHost(h.ID)
	if err != nil || got == nil || got.Name != "web-1" {
		t.Fatalf("get: %v %+v", err, got)
	}

	if err := st.UpdateHostSnapshot(h.ID, "Ubuntu 20.04", "5.4.0", false, 86400,
		[]models.HostDisk{{Mount: "/", Percent: 10}},
		[]models.HostServiceStatus{{Name: "nginx", Active: "active"}},
		&models.HostSecurity{LogsReadable: true, LastRootLogin: "2026-09-15T12:00:00Z"},
	); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetHost(h.ID)
	if err != nil || got == nil || got.UptimeSeconds != 86400 {
		t.Fatalf("uptime after snapshot: %v %+v", err, got)
	}
	if got.Security == nil || got.Security.LastRootLogin != "2026-09-15T12:00:00Z" {
		t.Fatalf("last root login: %+v", got.Security)
	}
	if err := st.UpdateHostSnapshot(h.ID, "Ubuntu 20.04", "5.4.0", false, 86400,
		[]models.HostDisk{{Mount: "/", Percent: 10}},
		[]models.HostServiceStatus{{Name: "nginx", Active: "active"}},
		&models.HostSecurity{LogsReadable: true},
	); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetHost(h.ID)
	if err != nil || got.Security == nil || got.Security.LastRootLogin != "2026-09-15T12:00:00Z" {
		t.Fatalf("last root login must stick: %+v", got.Security)
	}
	if len(got.ServiceStatus) != 1 || got.ServiceStatus[0].Name != "nginx" {
		t.Fatalf("services: %+v", got.ServiceStatus)
	}
	if err := st.UpdateHostSnapshot(h.ID, "", "", false, 0, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetHost(h.ID)
	if err != nil || got == nil || got.UptimeSeconds != 86400 {
		t.Fatalf("empty ingest must keep uptime: %v %+v", err, got)
	}
	if len(got.ServiceStatus) != 1 || got.ServiceStatus[0].Active != "active" {
		t.Fatalf("empty ingest must keep services: %+v", got.ServiceStatus)
	}

	token := "enroll-secret-token"
	expires := time.Now().UTC().Add(15 * time.Minute)
	if err := st.CreateHostEnrollToken(h.ID, token, expires); err != nil {
		t.Fatal(err)
	}
	host, err := st.ConsumeHostEnrollToken(token)
	if err != nil || host == nil || host.ID != h.ID {
		t.Fatalf("consume: %v %+v", err, host)
	}
	again, err := st.ConsumeHostEnrollToken(token)
	if err != nil || again != nil {
		t.Fatalf("single-use: %v %+v", err, again)
	}

	ingest := "ingest-secret-token"
	if err := st.SetHostIngestToken(h.ID, ingest); err != nil {
		t.Fatal(err)
	}
	byTok, err := st.GetHostByIngestToken(ingest)
	if err != nil || byTok == nil || byTok.ID != h.ID {
		t.Fatalf("ingest token: %v %+v", err, byTok)
	}
	wrong, err := st.GetHostByIngestToken("nope")
	if err != nil || wrong != nil {
		t.Fatalf("wrong token: %v %+v", err, wrong)
	}

	now := time.Now().UTC()
	if err := st.TouchHostSeen(h.ID, "web-1.local", "linux", "amd64", "1.0.0", 4, 3600, now); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetHost(h.ID)
	if err != nil || got == nil || got.UptimeSeconds != 3600 {
		t.Fatalf("uptime after seen: %v %+v", err, got)
	}
	cpu, mem := 12.5, 40.0
	if err := st.InsertHostSample(&models.HostSample{
		HostID: h.ID, CPUPercent: &cpu, MemPercent: &mem, NumCPU: 4,
		Disks: []models.HostDisk{{Mount: "/", Percent: 51}}, DiskPercent: floatPtr(51),
		CollectedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := st.GetHostStats(h.ID, now.Add(-time.Hour))
	if err != nil || len(stats.Points) != 1 || stats.Points[0].CPUPercent == nil {
		t.Fatalf("stats: %v %+v", err, stats)
	}
	if len(stats.Points[0].Disks) != 1 || stats.Points[0].Disks[0].Mount != "/" {
		t.Fatalf("disks: %+v", stats.Points[0].Disks)
	}

	stt, err := st.GetHostAlertState(h.ID, "cpu")
	if err != nil || stt.ConsecutiveHigh != 0 {
		t.Fatalf("alert state: %v %+v", err, stt)
	}
	stt.ConsecutiveHigh = 3
	if err := st.UpsertHostAlertState(stt); err != nil {
		t.Fatal(err)
	}
	stt2, err := st.GetHostAlertState(h.ID, "cpu")
	if err != nil || stt2.ConsecutiveHigh != 3 {
		t.Fatalf("upsert: %v %+v", err, stt2)
	}

	listed, err := st.ListHosts()
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %v %d", err, len(listed))
	}

	n, err := st.PruneOldHostSamples(now.Add(time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("prune: %v %d", err, n)
	}

	if err := st.DeleteHost(h.ID); err != nil {
		t.Fatal(err)
	}
	gone, err := st.GetHost(h.ID)
	if err != nil || gone != nil {
		t.Fatalf("delete: %v %+v", err, gone)
	}
}

func TestHostEnrollExpired(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := &models.Host{Name: "old", Enabled: true}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	token := "expired-token"
	if err := st.CreateHostEnrollToken(h.ID, token, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := st.ConsumeHostEnrollToken(token)
	if err != nil || got != nil {
		t.Fatalf("expired should miss: %v %+v", err, got)
	}
}

func TestMigrateV23ReplacesLegacyHostsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		DROP TABLE IF EXISTS host_alert_state;
		DROP TABLE IF EXISTS host_enroll_tokens;
		DROP TABLE IF EXISTS host_samples;
		DROP TABLE IF EXISTS hosts;
		CREATE TABLE hosts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			os_pretty_name TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO hosts (id, name) VALUES ('legacy', 'old');
	`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, err := st.GetHost("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("legacy host should be gone, got %+v", got)
	}
	h := &models.Host{Name: "fresh", Enabled: true}
	if err := st.CreateHost(h); err != nil {
		t.Fatal(err)
	}
}

func floatPtr(v float64) *float64 { return &v }
