package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestEmailLogInsertListPrune(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	old := models.EmailLog{
		Status:    models.EmailStatusSent,
		Kind:      models.EmailKindAlert,
		ToAddr:    "old@example.com",
		Subject:   "old",
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
	}
	if err := st.InsertEmailLog(&old); err != nil {
		t.Fatal(err)
	}
	fresh := models.EmailLog{
		Status:      models.EmailStatusFail,
		Kind:        models.EmailKindTest,
		ToAddr:      "new@example.com",
		Subject:     "new",
		Error:       "boom",
		MonitorName: "site",
	}
	if err := st.InsertEmailLog(&fresh); err != nil {
		t.Fatal(err)
	}

	items, err := st.QueryEmailLog(EmailLogQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("list=%d want 2", len(items))
	}
	n, err := st.CountEmailLog(EmailLogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d want 2", n)
	}

	failed, err := st.QueryEmailLog(EmailLogQuery{Status: models.EmailStatusFail, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 || failed[0].ToAddr != "new@example.com" {
		t.Fatalf("status filter=%v", failed)
	}

	pruned, err := st.PruneOldEmailLog(time.Now().UTC().Add(-24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Fatalf("pruned=%d want 1", pruned)
	}
	left, err := st.CountEmailLog(EmailLogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("after prune count=%d want 1", left)
	}
}
