package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func diagnoseAuthLogs() {
	now := time.Now()
	fmt.Printf("sentinel-agent auth diagnose %s\n", models.HostAgentVersion)
	printIdentity()

	fmt.Println()
	fmt.Println("== files ==")
	for _, path := range authLogPaths {
		fmt.Println(describeAuthFile(path))
	}

	fmt.Println()
	fmt.Println("== journal probe ==")
	bin := journalctlPath()
	if bin == "" {
		fmt.Println("journalctl: not found")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, stderr, err := runJournalctl(ctx, bin, "--system", "--no-pager", "-n", "1", "-o", "short")
		cancel()
		if err != nil {
			fmt.Printf("journalctl --system -n 1: err=%v stderr=%s\n", err, strings.TrimSpace(stderr))
		} else {
			fmt.Printf("journalctl --system -n 1: ok\n%s\n", strings.TrimSpace(string(out)))
		}
		fmt.Println()
		fmt.Println("== last 20 matching journal lines (5m) ==")
		printJournalMatches(now.Add(-5*time.Minute), now, 20)
	}

	fmt.Println()
	fmt.Println("== classified (5m) ==")
	sec := collectSecurity(now, 5*time.Minute)
	fmt.Printf("logs_readable=%v source=%s\n", sec.LogsReadable, sec.LogSource)
	if sec.LogError != "" {
		fmt.Printf("notes: %s\n", sec.LogError)
	}
	fmt.Printf("ssh_failed=%d sudo_failed=%d auth_failed=%d root_logins=%d last_root=%s\n",
		sec.SSHFailed5m, sec.SudoFailed5m, sec.AuthFailed5m, sec.RootLogins5m, sec.LastRootLogin)
}

func printIdentity() {
	fmt.Printf("uid=%d euid=%d gid=%d\n", os.Getuid(), os.Geteuid(), os.Getgid())
	if gids, err := os.Getgroups(); err == nil {
		fmt.Printf("groups=%v\n", gids)
	}
	if out, err := exec.Command("/usr/bin/id").Output(); err == nil {
		fmt.Printf("id: %s", out)
	}
}

func describeAuthFile(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return path + ": " + err.Error()
	}
	owner := ""
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		owner = fmt.Sprintf(" uid=%d gid=%d", st.Uid, st.Gid)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("%s: exists mode=%s%s open: %v", path, fi.Mode(), owner, err)
	}
	f.Close()
	return fmt.Sprintf("%s: readable mode=%s%s", path, fi.Mode(), owner)
}

func printJournalMatches(since, now time.Time, limit int) {
	bin := journalctlPath()
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{
		"--system", "--no-pager", "-o", "json",
		"-n", fmt.Sprintf("%d", limit),
		"--since", since.UTC().Format(time.RFC3339),
	}
	args = append(args, journalctlArgsOR...)
	out, stderr, err := runJournalctl(ctx, bin, args...)
	if journalPermissionDenied(err, stderr) {
		fmt.Printf("journal matches: permission denied: %s\n", strings.TrimSpace(stderr))
		return
	}
	if err != nil && !journalctlNoEntries(err) && len(out) == 0 {
		fmt.Printf("journal matches: err=%v stderr=%s\n", err, strings.TrimSpace(stderr))
		return
	}
	if len(out) == 0 {
		fmt.Println("(no matching lines in window)")
		return
	}
	n := 0
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var rec struct {
			Message json.RawMessage `json:"MESSAGE"`
			Ident   string          `json:"SYSLOG_IDENTIFIER"`
			Comm    string          `json:"_COMM"`
		}
		if err := dec.Decode(&rec); err != nil {
			break
		}
		msg := decodeJournalString(rec.Message)
		if msg == "" {
			continue
		}
		n++
		if len(msg) > 240 {
			msg = msg[:240] + "…"
		}
		src := rec.Ident
		if src == "" {
			src = rec.Comm
		}
		fmt.Printf("[%s] %s  %s\n", classifyTag(msg), src, msg)
	}
	if n == 0 {
		fmt.Println("(no MESSAGE fields in journal JSON)")
	}
}

func classifyTag(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case isSSHFailure(lower):
		return "ssh"
	case isSudoFailure(lower):
		return "sudo"
	case isAuthFailure(lower):
		return "auth"
	}
	if isRootLogin(lower) {
		return "root"
	}
	return "-"
}
