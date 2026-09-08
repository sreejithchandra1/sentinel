package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

var authLogPaths = []string{"/var/log/auth.log", "/var/log/secure"}

func collectSecurity(now time.Time, window time.Duration) *models.HostSecurity {
	sec := &models.HostSecurity{}
	cutoff := now.Add(-window)
	for _, path := range authLogPaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		parseAuthLog(f, cutoff, now, sec)
		f.Close()
		sec.LogsReadable = true
		return sec
	}
	return sec
}

func parseAuthLog(r io.Reader, cutoff, now time.Time, sec *models.HostSecurity) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		ts, msg, ok := splitLogLine(line, now)
		if !ok || ts.Before(cutoff) {
			continue
		}
		classifyAuthLine(msg, ts, sec)
	}
}

func classifyAuthLine(msg string, ts time.Time, sec *models.HostSecurity) {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "failed password"), strings.Contains(lower, "failed publickey"),
		strings.Contains(lower, "invalid user") && strings.Contains(lower, "sshd"):
		sec.SSHFailed5m++
	case strings.Contains(lower, "sudo:") && (strings.Contains(lower, "authentication failure") ||
		strings.Contains(lower, "incorrect password") || strings.Contains(lower, "not in the sudoers")):
		sec.SudoFailed5m++
	case strings.Contains(lower, "authentication failure"):
		sec.AuthFailed5m++
	}
	if isRootLogin(lower) {
		sec.RootLogins5m++
		sec.LastRootLogin = ts.UTC().Format(time.RFC3339)
	}
}

func isRootLogin(lower string) bool {
	if strings.Contains(lower, "accepted password for root") ||
		strings.Contains(lower, "accepted publickey for root") ||
		strings.Contains(lower, "session opened for user root") {
		return true
	}
	return false
}

func splitLogLine(line string, now time.Time) (time.Time, string, bool) {
	if len(line) < 15 {
		return time.Time{}, "", false
	}
	// RFC3339 / ISO journal prefixes
	if line[4] == '-' && line[7] == '-' {
		fields := strings.SplitN(line, " ", 2)
		if t, err := time.Parse(time.RFC3339, strings.TrimSuffix(fields[0], ":")); err == nil {
			msg := line
			if len(fields) > 1 {
				msg = fields[1]
			}
			return t, msg, true
		}
		if t, err := time.Parse("2006-01-02T15:04:05", fields[0]); err == nil {
			return t, fields[1], true
		}
	}
	// syslog: "Sep  8 21:15:01 host rest..."
	if len(line) < 16 {
		return time.Time{}, "", false
	}
	stamp := line[:15]
	t, err := time.Parse("Jan  2 15:04:05", stamp)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05", strings.TrimSpace(stamp))
		if err != nil {
			return time.Time{}, "", false
		}
	}
	t = t.AddDate(now.Year(), 0, 0)
	t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
	if t.After(now.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, line[16:], true
}
