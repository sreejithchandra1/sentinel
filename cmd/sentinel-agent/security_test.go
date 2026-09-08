package main

import (
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

func TestNormalizeWatchedServices(t *testing.T) {
	got := models.NormalizeWatchedServices([]string{" nginx ", "nginx.service", "sshd", "bad name", "ok-unit"})
	if len(got) != 3 || got[0] != "nginx" || got[1] != "sshd" || got[2] != "ok-unit" {
		t.Fatalf("got %#v", got)
	}
}
