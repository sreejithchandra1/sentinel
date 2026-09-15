package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestParseAuthLogCounts(t *testing.T) {
	now := time.Date(2026, 9, 8, 21, 20, 0, 0, time.UTC)
	log := strings.Join([]string{
		"Sep  8 21:16:01 web sshd[1]: Failed password for invalid user admin from 1.2.3.4 port 22 ssh2",
		"Sep  8 21:16:02 web sshd[1]: Failed password for root from 1.2.3.4 port 22 ssh2",
		"Sep  8 21:16:03 web sudo: pam_unix(sudo:auth): authentication failure; logname= uid=1000 euid=0 tty=/dev/pts/0 ruser=alice rhost=  user=alice",
		"Sep  8 21:16:04 web sshd[2]: Accepted publickey for root from 10.0.0.1 port 22 ssh2",
		"Sep  8 20:00:00 web sshd[3]: Failed password for nobody from 1.2.3.4 port 22 ssh2",
	}, "\n")
	sec := &models.HostSecurity{}
	parseAuthLog(strings.NewReader(log), now.Add(-5*time.Minute), now, sec)
	if sec.SSHFailed5m != 2 {
		t.Fatalf("ssh=%d, want 2", sec.SSHFailed5m)
	}
	if sec.SudoFailed5m != 1 {
		t.Fatalf("sudo=%d, want 1", sec.SudoFailed5m)
	}
	if sec.RootLogins5m != 1 {
		t.Fatalf("root=%d, want 1", sec.RootLogins5m)
	}
}

func TestIsRootLoginAlmaLines(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"Accepted keyboard-interactive/pam for root from 10.0.0.1 port 22 ssh2", true},
		{"pam_unix(sshd:session): session opened for user root(uid=0) by (uid=0)", true},
		{"Accepted publickey for root from 10.0.0.1 port 22 ssh2", true},
		{"Failed password for root from 1.2.3.4 port 22 ssh2", false},
		{"Accepted publickey for alma from 10.0.0.1 port 22 ssh2", false},
	}
	for _, tc := range cases {
		if got := isRootLogin(strings.ToLower(tc.line)); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.line, got, tc.want)
		}
	}
}

func TestParseJournalJSON(t *testing.T) {
	now := time.Date(2026, 9, 15, 7, 30, 0, 0, time.UTC)
	cutoff := now.Add(-5 * time.Minute)
	oldTS := strconv.FormatInt(cutoff.Add(-time.Minute).UnixMicro(), 10)
	newTS := strconv.FormatInt(now.Add(-time.Minute).UnixMicro(), 10)
	raw := strings.Join([]string{
		`{"MESSAGE":"Failed password for invalid user admin from 1.2.3.4 port 22 ssh2","__REALTIME_TIMESTAMP":"` + newTS + `"}`,
		`{"MESSAGE":"Accepted publickey for root from 10.0.0.1 port 22 ssh2","__REALTIME_TIMESTAMP":"` + newTS + `"}`,
		`{"MESSAGE":"Failed password for nobody from 1.2.3.4 port 22 ssh2","__REALTIME_TIMESTAMP":"` + oldTS + `"}`,
	}, "\n")
	sec := &models.HostSecurity{}
	parseJournalJSON(strings.NewReader(raw), cutoff, now, sec)
	if sec.SSHFailed5m != 1 {
		t.Fatalf("ssh=%d, want 1", sec.SSHFailed5m)
	}
	if sec.RootLogins5m != 1 {
		t.Fatalf("root=%d, want 1", sec.RootLogins5m)
	}
}

func TestParseJournalJSONByteMessageAndNumericTS(t *testing.T) {
	now := time.Date(2026, 9, 15, 7, 30, 0, 0, time.UTC)
	cutoff := now.Add(-5 * time.Minute)
	us := now.Add(-time.Minute).UnixMicro()
	msg := "Failed password for root from 1.2.3.4 port 22 ssh2"
	var nums []string
	for i := 0; i < len(msg); i++ {
		nums = append(nums, strconv.Itoa(int(msg[i])))
	}
	raw := `{"MESSAGE":[` + strings.Join(nums, ",") + `],"__REALTIME_TIMESTAMP":` + strconv.FormatInt(us, 10) + `}`
	sec := &models.HostSecurity{}
	parseJournalJSON(strings.NewReader(raw), cutoff, now, sec)
	if sec.SSHFailed5m != 1 {
		t.Fatalf("ssh=%d, want 1", sec.SSHFailed5m)
	}
}

func TestJournalPermissionDenied(t *testing.T) {
	if !journalPermissionDenied(nil, "No journal files were opened due to insufficient permissions.\n") {
		t.Fatal("expected permission denied")
	}
	if journalPermissionDenied(nil, "") {
		t.Fatal("empty stderr is not a permission error")
	}
}

func TestNormalizeWatchedServices(t *testing.T) {
	got := models.NormalizeWatchedServices([]string{" nginx ", "nginx.service", "sshd", "bad name", "ok-unit"})
	if len(got) != 3 || got[0] != "nginx" || got[1] != "sshd" || got[2] != "ok-unit" {
		t.Fatalf("got %#v", got)
	}
}
