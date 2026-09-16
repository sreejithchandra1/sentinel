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
	authLogPaths    = []string{"/var/log/auth.log", "/var/log/secure"}
	journalFailOnce sync.Once
	// Same style as fillLastRootLogin, which already works on Alma: relative/ISO
	// timestamps that systemd 239+ parse, plus --grep. RFC3339 "...Z" and a lone
	// "+" match argument both produced zero hits on RHEL journalctl.
	journalAuthGrep = `(?i)failed password|failed publickey|invalid user|authentication failure|password check failed|not in sudoers|incorrect password|connection closed by|session opened for user root|accepted .+ for root|failed keyboard-interactive|failed none for|may not run sudo|not allowed to run sudo`
)

func collectSecurity(now time.Time, window time.Duration) *models.HostSecurity {
	sec := &models.HostSecurity{}
	cutoff := now.Add(-window)
	var fileNotes []string
	for _, path := range authLogPaths {
		f, err := os.Open(path)
		if err != nil {
			fileNotes = append(fileNotes, path+": "+err.Error())
			continue
		}
		parseAuthLog(f, cutoff, now, sec)
		f.Close()
		sec.LogsReadable = true
		sec.LogSource = "file:" + path
		if len(fileNotes) > 0 {
			sec.LogError = strings.Join(fileNotes, "; ")
		}
		return sec
	}
	// RHEL/Alma/Rocky: /var/log/secure is root:root 0600. Read the systemd journal instead.
	if collectSecurityJournal(cutoff, now, sec) {
		sec.LogsReadable = true
		sec.LogSource = "journal"
		if len(fileNotes) > 0 {
			sec.LogError = strings.Join(fileNotes, "; ")
		}
		return sec
	}
	if len(fileNotes) > 0 {
		sec.LogError = strings.Join(append(fileNotes, "journal: not readable"), "; ")
	} else {
		sec.LogError = "journal: not readable"
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

	args := journalAuthQueryArgs(cutoff, 5000)
	out, stderr, err := runJournalctl(ctx, bin, args...)
	if journalPermissionDenied(err, stderr) {
		journalFailOnce.Do(func() { log.Printf("security: journal permission denied: %s", strings.TrimSpace(stderr)) })
		return false
	}
	if journalctlBadSince(stderr) {
		journalFailOnce.Do(func() { log.Printf("security: journalctl --since: %s", strings.TrimSpace(stderr)) })
	}
	if len(out) > 0 {
		parseJournalJSON(bytes.NewReader(out), cutoff, now, sec)
	}
	fillLastRootLogin(sec)
	return true
}

func journalAuthQueryArgs(since time.Time, lines int) []string {
	return []string{
		"--system", "--no-pager", "-o", "json",
		"-n", strconv.Itoa(lines),
		"--since", journalSince(since),
		"--grep", journalAuthGrep,
	}
}

func journalSince(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}

func journalctlBadSince(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "failed to parse") || strings.Contains(s, "invalid timestamp")
}

func fillLastRootLogin(sec *models.HostSecurity) {
	bin := journalctlPath()
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, _, err := runJournalctl(ctx, bin,
		"--system", "--no-pager", "-o", "json", "-n", "200",
		"--since", "30 days ago",
		"--grep", `(?i)session opened for user root|accepted .+ for root`,
	)
	if err != nil && !journalctlNoEntries(err) {
		return
	}
	if len(out) == 0 {
		return
	}
	tmp := &models.HostSecurity{}
	parseJournalJSON(bytes.NewReader(out), time.Time{}, time.Now().UTC(), tmp)
	if tmp.LastRootLogin == "" {
		return
	}
	if sec.LastRootLogin == "" || tmp.LastRootLogin > sec.LastRootLogin {
		sec.LastRootLogin = tmp.LastRootLogin
	}
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
	case isSSHFailure(lower):
		sec.SSHFailed5m++
	case isSudoFailure(lower):
		sec.SudoFailed5m++
	case isAuthFailure(lower):
		sec.AuthFailed5m++
	}
	if isRootLogin(lower) {
		sec.RootLogins5m++
		sec.LastRootLogin = ts.UTC().Format(time.RFC3339)
	}
}

func isSSHFailure(lower string) bool {
	if strings.Contains(lower, "failed password") || strings.Contains(lower, "failed publickey") {
		return true
	}
	if strings.Contains(lower, "invalid user") {
		return true
	}
	if strings.Contains(lower, "connection closed by authenticating user") {
		return true
	}
	if strings.Contains(lower, "failed keyboard-interactive") || strings.Contains(lower, "failed none for") {
		return true
	}
	return false
}

func isSudoFailure(lower string) bool {
	if strings.Contains(lower, "not in sudoers") || strings.Contains(lower, "not in the sudoers") {
		return true
	}
	if strings.Contains(lower, "incorrect password attempt") {
		return true
	}
	if strings.Contains(lower, "may not run sudo") || strings.Contains(lower, "not allowed to run sudo") {
		return true
	}
	// RHEL: pam_unix(sudo-i:auth): authentication failure  (journal MESSAGE has no "sudo:" prefix)
	if strings.Contains(lower, "authentication failure") &&
		(strings.Contains(lower, "sudo-i:auth") || strings.Contains(lower, "sudo:auth") ||
			strings.Contains(lower, "pam_unix(sudo") || strings.Contains(lower, "sudo[") ||
			strings.Contains(lower, "sudo:")) {
		return true
	}
	if (strings.Contains(lower, "sudo:") || strings.Contains(lower, "sudo[")) &&
		strings.Contains(lower, "incorrect password") {
		return true
	}
	return false
}

func isAuthFailure(lower string) bool {
	if strings.Contains(lower, "authentication failure") {
		return true
	}
	// RHEL unix_chkpwd: "password check failed for user (twohats)"
	if strings.Contains(lower, "password check failed") {
		return true
	}
	return false
}

func isRootLogin(lower string) bool {
	if strings.Contains(lower, "failed") || strings.Contains(lower, "invalid user") ||
		strings.Contains(lower, "authentication failure") {
		return false
	}
	if strings.Contains(lower, "session opened for user root") {
		return true
	}
	// Alma/RHEL: "Accepted keyboard-interactive/pam for root from …"
	if strings.Contains(lower, "accepted ") && strings.Contains(lower, " for root") {
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
