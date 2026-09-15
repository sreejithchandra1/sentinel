package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

var (
	authLogPaths     = []string{"/var/log/auth.log", "/var/log/secure"}
	journalFailOnce  sync.Once
	journalctlArgsOR = []string{
		"SYSLOG_IDENTIFIER=sshd",
		"SYSLOG_IDENTIFIER=sshd-session",
		"SYSLOG_IDENTIFIER=sudo",
		"SYSLOG_IDENTIFIER=su",
		"SYSLOG_IDENTIFIER=login",
		"+", "_COMM=sshd",
		"+", "_COMM=sudo",
		"+", "_SYSTEMD_UNIT=sshd.service",
		"+", "SYSLOG_FACILITY=4",
		"+", "SYSLOG_FACILITY=10",
	}
)

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
	// RHEL/Alma/Rocky: /var/log/secure is root:root 0600. Read the systemd journal instead.
	if collectSecurityJournal(cutoff, now, sec) {
		sec.LogsReadable = true
	}
	return sec
}

func collectSecurityJournal(cutoff, now time.Time, sec *models.HostSecurity) bool {
	bin := journalctlPath()
	if bin == "" {
		journalFailOnce.Do(func() { log.Printf("security: journalctl not found") })
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Probe: can we see the system journal at all? --quiet + no matches exits 1.
	if !journalIsReadable(ctx, bin) {
		return false
	}

	args := []string{
		"--system", "--no-pager", "-o", "json", "-n", "5000",
		"--since", cutoff.UTC().Format(time.RFC3339),
	}
	args = append(args, journalctlArgsOR...)
	out, stderr, err := runJournalctl(ctx, bin, args...)
	if journalPermissionDenied(err, stderr) {
		journalFailOnce.Do(func() { log.Printf("security: journal permission denied: %s", strings.TrimSpace(stderr)) })
		return false
	}
	if len(out) > 0 {
		parseJournalJSON(bytes.NewReader(out), cutoff, now, sec)
	}
	return true
}

func journalIsReadable(ctx context.Context, bin string) bool {
	_, stderr, err := runJournalctl(ctx, bin, "--system", "--no-pager", "-n", "1", "-o", "json")
	if journalPermissionDenied(err, stderr) {
		journalFailOnce.Do(func() { log.Printf("security: journal not readable: %s", strings.TrimSpace(stderr)) })
		return false
	}
	if err == nil || journalctlNoEntries(err) {
		return true
	}
	if ctx.Err() != nil {
		journalFailOnce.Do(func() { log.Printf("security: journalctl timed out") })
		return false
	}
	journalFailOnce.Do(func() { log.Printf("security: journalctl: %v %s", err, strings.TrimSpace(stderr)) })
	return false
}

func runJournalctl(ctx context.Context, bin string, args ...string) (stdout []byte, stderr string, err error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return out.Bytes(), errBuf.String(), err
}

func journalPermissionDenied(err error, stderr string) bool {
	s := strings.ToLower(stderr)
	if err != nil {
		s += " " + strings.ToLower(err.Error())
	}
	return strings.Contains(s, "insufficient permissions") ||
		strings.Contains(s, "no journal files were opened") ||
		strings.Contains(s, "permission denied")
}

func journalctlNoEntries(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 1
}

func journalctlPath() string {
	for _, p := range []string{"/usr/bin/journalctl", "/bin/journalctl"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	p, err := exec.LookPath("journalctl")
	if err != nil {
		return ""
	}
	return p
}

func parseJournalJSON(r io.Reader, cutoff, now time.Time, sec *models.HostSecurity) {
	dec := json.NewDecoder(r)
	for {
		var rec struct {
			Message json.RawMessage `json:"MESSAGE"`
			TS      json.RawMessage `json:"__REALTIME_TIMESTAMP"`
		}
		if err := dec.Decode(&rec); err != nil {
			return
		}
		msg := decodeJournalString(rec.Message)
		ts := now
		if t, ok := decodeJournalTimestamp(rec.TS); ok {
			ts = t
		}
		if ts.Before(cutoff) {
			continue
		}
		classifyAuthLine(msg, ts, sec)
	}
}

func decodeJournalString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var nums []float64
	if err := json.Unmarshal(raw, &nums); err == nil {
		b := make([]byte, len(nums))
		for i, n := range nums {
			b[i] = byte(n)
		}
		return string(b)
	}
	return ""
}

func decodeJournalTimestamp(raw json.RawMessage) (time.Time, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if us, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(0, us*int64(time.Microsecond)).UTC(), true
		}
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if us, err := n.Int64(); err == nil {
			return time.Unix(0, us*int64(time.Microsecond)).UTC(), true
		}
	}
	return time.Time{}, false
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
