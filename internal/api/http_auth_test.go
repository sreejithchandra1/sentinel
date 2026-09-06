package api

import (
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

func TestApplyHTTPAuthUpdateKeepsPasswordWhenBlank(t *testing.T) {
	existing := &models.Monitor{HTTPUsername: "alice", HTTPPassword: "old"}
	applyHTTPAuthUpdate(existing, &models.Monitor{HTTPUsername: "alice", HTTPPassword: ""})
	if existing.HTTPUsername != "alice" || existing.HTTPPassword != "old" {
		t.Fatalf("got user=%q pass=%q", existing.HTTPUsername, existing.HTTPPassword)
	}
}

func TestApplyHTTPAuthUpdateClearsWhenUsernameEmpty(t *testing.T) {
	existing := &models.Monitor{HTTPUsername: "alice", HTTPPassword: "old"}
	applyHTTPAuthUpdate(existing, &models.Monitor{HTTPUsername: "  ", HTTPPassword: "x"})
	if existing.HTTPUsername != "" || existing.HTTPPassword != "" {
		t.Fatalf("expected cleared auth, got user=%q pass=%q", existing.HTTPUsername, existing.HTTPPassword)
	}
}

func TestApplyPerformanceHTTPAuthUpdateKeepsPasswordWhenBlank(t *testing.T) {
	existing := &models.PerformanceTarget{HTTPUsername: "alice", HTTPPassword: "old"}
	applyPerformanceHTTPAuthUpdate(existing, &models.PerformanceTarget{HTTPUsername: "alice", HTTPPassword: ""})
	if existing.HTTPUsername != "alice" || existing.HTTPPassword != "old" {
		t.Fatalf("got user=%q pass=%q", existing.HTTPUsername, existing.HTTPPassword)
	}
}

func TestSanitizeMonitorHTTPAuthHidesPassword(t *testing.T) {
	m := &models.Monitor{HTTPUsername: "alice", HTTPPassword: "secret"}
	sanitizeMonitorHTTPAuth(m, &models.User{Role: models.RoleAdmin, TenantID: "t1"})
	if m.HTTPPassword != "" {
		t.Fatal("password should be stripped")
	}
	if !m.HTTPAuthSet || m.HTTPUsername != "alice" {
		t.Fatalf("auth_set=%v user=%q", m.HTTPAuthSet, m.HTTPUsername)
	}
}
