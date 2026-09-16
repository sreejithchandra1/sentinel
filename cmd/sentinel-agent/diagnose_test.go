package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDescribeAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secure")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := describeAuthFile(path)
	if !strings.Contains(got, "readable") {
		t.Fatalf("want readable, got %q", got)
	}
	missing := describeAuthFile(filepath.Join(dir, "nope"))
	if !strings.Contains(missing, "nope") {
		t.Fatalf("want missing path, got %q", missing)
	}
}

func TestClassifyTagAlmaLines(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"Connection closed by invalid user co 216.87.32.78 port 17403 [preauth]", "ssh"},
		{"password check failed for user (twohats)", "auth"},
		{"pam_unix(sudo-i:auth): authentication failure; logname=root uid=10006", "sudo"},
		{"Accepted keyboard-interactive/pam for root from 10.0.0.1 port 22 ssh2", "root"},
	}
	for _, tc := range cases {
		if got := classifyTag(tc.msg); got != tc.want {
			t.Fatalf("%q: got %s want %s", tc.msg, got, tc.want)
		}
	}
}

func TestJournalSinceAndQueryArgs(t *testing.T) {
	ts := time.Date(2026, 9, 15, 10, 47, 57, 0, time.UTC)
	if got := journalSince(ts); got != "2026-09-15 10:47:57 UTC" {
		t.Fatalf("journalSince=%q", got)
	}
	args := journalAuthQueryArgs(ts, 20)
	joined := strings.Join(args, "\x00")
	if strings.Contains(joined, "2026-09-15T") || strings.Contains(joined, "T10:47:57Z") {
		t.Fatalf("RFC3339 timestamp leaked into journalctl args: %q", args)
	}
	for _, a := range args {
		if a == "+" {
			t.Fatal("lone + journal match must not be passed")
		}
	}
}
