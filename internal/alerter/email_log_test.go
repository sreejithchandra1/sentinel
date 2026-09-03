package alerter

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/models"
	"github.com/sentinel-monitoring/sentinel/internal/store"
)

func TestHandleResult_PendingWritesEmailLog(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := &models.Monitor{
		Name:               "site",
		Type:               models.MonitorHTTP,
		URL:                "https://example.com",
		IntervalSeconds:    60,
		TimeoutMs:          5000,
		Enabled:            true,
		NotifyEmail:        true,
		AlertAfterFailures: 2,
		LastStatus:         models.StatusUp,
	}
	if err := st.CreateMonitor(m); err != nil {
		t.Fatal(err)
	}

	a := New(st, models.SMTPConfig{Host: "smtp.example.com", From: "alerts@example.com", Enabled: true, AlertEmails: "ops@example.com"}, models.SMTPConfig{}, "http://localhost")
	a.notifyHook = func(*models.Monitor, string, string, int) error { return nil }

	if err := a.HandleResult(m, &models.CheckResult{
		MonitorID: m.ID, Status: models.StatusDown, Error: "timeout", CheckedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	items, err := st.QueryEmailLog(store.EmailLogQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("log=%v want 1 pending row", items)
	}
	if items[0].Status != models.EmailStatusPending {
		t.Fatalf("status=%s want pending", items[0].Status)
	}
	if items[0].MonitorName != "site" {
		t.Fatalf("monitor=%s", items[0].MonitorName)
	}
}

func TestSendSMTP_SkipAndFailWriteLog(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a := New(st, models.SMTPConfig{}, models.SMTPConfig{}, "http://localhost")
	if err := a.sendSMTP("ops@example.com", "subj", "<p>x</p>", smtpLogMeta{Kind: models.EmailKindTest}); err == nil {
		t.Fatal("expected skip when host empty")
	}
	items, err := st.QueryEmailLog(store.EmailLogQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != models.EmailStatusSkip {
		t.Fatalf("skip log=%v", items)
	}

	a.cfg = models.SMTPConfig{Host: "127.0.0.1", Port: 1, From: "alerts@example.com"}
	if err := a.sendSMTP("ops@example.com", "subj", "<p>x</p>", smtpLogMeta{Kind: models.EmailKindAlert}); err == nil {
		t.Fatal("expected fail on refused port")
	}
	items, err = st.QueryEmailLog(store.EmailLogQuery{Status: models.EmailStatusFail, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("fail log=%v", items)
	}
}

func TestSendSMTP_SentWritesLog(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	host, port, stop := startFakeSMTP(t)
	defer stop()

	a := New(st, models.SMTPConfig{Host: host, Port: port, From: "alerts@example.com", TLS: false}, models.SMTPConfig{}, "http://localhost")
	if err := a.sendSMTP("ops@example.com", "[Sentinel] Test Email", "<p>hi</p>", smtpLogMeta{Kind: models.EmailKindTest}); err != nil {
		t.Fatal(err)
	}
	items, err := st.QueryEmailLog(store.EmailLogQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != models.EmailStatusSent {
		t.Fatalf("sent log=%v", items)
	}
	if items[0].ToAddr != "ops@example.com" || items[0].Kind != models.EmailKindTest {
		t.Fatalf("row=%+v", items[0])
	}
}

func startFakeSMTP(t *testing.T) (host string, port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go serveFakeSMTP(conn)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, func() {
		close(done)
		_ = ln.Close()
	}
}

func serveFakeSMTP(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)
	_, _ = io.WriteString(conn, "220 localhost ESMTP test\r\n")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO") || strings.HasPrefix(cmd, "HELO"):
			_, _ = io.WriteString(conn, "250-localhost\r\n250 OK\r\n")
		case strings.HasPrefix(cmd, "MAIL"):
			_, _ = io.WriteString(conn, "250 OK\r\n")
		case strings.HasPrefix(cmd, "RCPT"):
			_, _ = io.WriteString(conn, "250 OK\r\n")
		case strings.HasPrefix(cmd, "DATA"):
			_, _ = io.WriteString(conn, "354 End data with <CR><LF>.<CR><LF>\r\n")
			for {
				l, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(l, "\r\n") == "." {
					break
				}
			}
			_, _ = io.WriteString(conn, "250 OK\r\n")
		case strings.HasPrefix(cmd, "QUIT"):
			_, _ = io.WriteString(conn, "221 Bye\r\n")
			return
		default:
			_, _ = fmt.Fprintf(conn, "250 OK\r\n")
		}
	}
}
